import { rm } from "node:fs/promises";
import { resolve } from "node:path";
import {
  readDeploymentReceipt,
  type DeployFailure,
  type DeploymentMode,
  type ProjectConfig,
  type ProviderAdapter,
  type StateAddress,
  type StateBackend,
  type StateLockInfo
} from "@runfabric/core";
import { getProviderPackageName } from "../../providers/registry";
import { info, warn } from "../../utils/logger";
import type {
  DeployCollections,
  DeployContext,
  DeployWorkflowInput,
  LockHeartbeat,
  ProviderDeployInput,
  ProviderStateSession
} from "./workflow-types";

function stringifyError(errorValue: unknown): string {
  if (errorValue instanceof Error) {
    return errorValue.message;
  }
  return String(errorValue);
}

function compactStringMap(
  value: Record<string, string> | undefined
): Record<string, string> | undefined {
  if (!value) {
    return undefined;
  }
  const out: Record<string, string> = {};
  for (const [key, entryValue] of Object.entries(value)) {
    if (typeof entryValue !== "string" || entryValue.trim().length === 0) {
      continue;
    }
    out[key] = entryValue.trim();
  }
  return Object.keys(out).length > 0 ? out : undefined;
}

function logProgress(enabled: boolean | undefined, message: string): void {
  if (enabled) {
    info(message);
  }
}

function startLockHeartbeat(
  stateBackend: StateBackend,
  address: StateAddress,
  lock: StateLockInfo
): LockHeartbeat {
  const intervalMs = Math.max(1, stateBackend.config.lock.heartbeatSeconds) * 1000;
  let active = true;
  let currentLock = lock;
  let inFlight = false;

  const timer = setInterval(() => {
    if (!active || inFlight || lock.lockId === "locks-disabled") {
      return;
    }
    inFlight = true;
    void stateBackend
      .renewLock(address, currentLock)
      .then((renewed) => {
        currentLock = renewed;
      })
      .catch(() => undefined)
      .finally(() => {
        inFlight = false;
      });
  }, intervalMs);
  timer.unref();

  return {
    latestLock() {
      return currentLock;
    },
    async stop(): Promise<void> {
      active = false;
      clearInterval(timer);
    }
  };
}

function pushMissingAdapterFailure(
  artifactProvider: string,
  failures: DeployFailure[]
): void {
  const packageName = getProviderPackageName(artifactProvider);
  failures.push({
    provider: artifactProvider,
    phase: "provider",
    message: packageName
      ? `provider adapter is not installed (${packageName})`
      : "provider adapter is not installed"
  });
}

async function validateProvider(
  provider: ProviderAdapter,
  project: ProjectConfig,
  failures: DeployFailure[]
): Promise<boolean> {
  const validation = await provider.validate(project);
  if (validation.ok) {
    return true;
  }
  for (const providerError of validation.errors || []) {
    failures.push({ provider: provider.name, phase: "validation", message: providerError });
  }
  return false;
}

function createStateAddress(project: ProjectConfig, stage: string, provider: string): StateAddress {
  return {
    service: project.service,
    stage,
    provider
  };
}

function createInProgressStateDetails(
  artifact: ProviderDeployInput["artifact"],
  functionName: string | undefined,
  startedAt: string,
  retryFromInProgress: boolean
): Record<string, unknown> {
  return {
    artifact,
    functionName,
    retryFromInProgress,
    startedAt
  };
}

async function openProviderStateSession(
  params: ProviderDeployInput,
  provider: ProviderAdapter
): Promise<ProviderStateSession> {
  const { context, input } = params;
  const stateAddress = createStateAddress(context.project, context.stage, provider.name);
  const existing = await context.stateBackend.read(stateAddress);

  if (existing?.lifecycle === "in_progress") {
    warn(`${provider.name}: previous deployment state was in_progress; continuing with idempotent retry`);
  }

  const stateLock = await context.stateBackend.lock(
    stateAddress,
    `deploy:${process.pid}:${provider.name}:${context.project.service}:${context.stage}`
  );
  const heartbeat = startLockHeartbeat(context.stateBackend, stateAddress, stateLock);
  const deploymentStartedAt = new Date().toISOString();

  await context.stateBackend.write(
    stateAddress,
    {
      schemaVersion: 2,
      provider: provider.name,
      service: context.project.service,
      stage: context.stage,
      endpoint: existing?.endpoint,
      lifecycle: "in_progress",
      updatedAt: deploymentStartedAt,
      details: createInProgressStateDetails(
        params.artifact,
        input.functionName,
        deploymentStartedAt,
        existing?.lifecycle === "in_progress"
      )
    },
    heartbeat.latestLock()
  );

  return { stateAddress, stateLock, heartbeat, deploymentStartedAt };
}

function mergeAddresses(
  primary?: Record<string, string>,
  secondary?: Record<string, string>
): Record<string, string> | undefined {
  return compactStringMap({ ...(primary || {}), ...(secondary || {}) });
}

async function runProviderDeployment(
  params: ProviderDeployInput,
  provider: ProviderAdapter
): Promise<{
  endpoint?: string;
  mode?: DeploymentMode;
  resourceAddresses?: Record<string, string>;
  workflowAddresses?: Record<string, string>;
  secretReferences?: Record<string, string>;
  deployPlan: Awaited<ReturnType<ProviderAdapter["planDeploy"]>>;
}> {
  const { context, artifact, input } = params;

  const provisionedResources = provider.provisionResources
    ? await provider.provisionResources(context.project)
    : undefined;
  const deployedWorkflows = provider.deployWorkflows
    ? await provider.deployWorkflows(context.project)
    : undefined;
  const materializedSecrets = provider.materializeSecrets
    ? await provider.materializeSecrets(context.project)
    : undefined;

  const providerBuildPlan = await provider.planBuild(context.project);
  await provider.build(context.project, providerBuildPlan);
  const deployPlan = await provider.planDeploy(context.project, artifact);
  const deployResult = await provider.deploy(context.project, deployPlan);

  const receipt = await readDeploymentReceipt(input.projectDir, provider.name);
  return {
    endpoint: deployResult.endpoint,
    mode: receipt?.mode,
    deployPlan,
    resourceAddresses: mergeAddresses(
      provisionedResources?.resourceAddresses,
      deployResult.resourceAddresses
    ),
    workflowAddresses: mergeAddresses(
      deployedWorkflows?.workflowAddresses,
      deployResult.workflowAddresses
    ),
    secretReferences: mergeAddresses(
      materializedSecrets?.secretReferences,
      deployResult.secretReferences
    )
  };
}

function createAppliedStateDetails(
  params: ProviderDeployInput,
  deployPlan: Awaited<ReturnType<ProviderAdapter["planDeploy"]>>,
  deploymentStartedAt: string,
  addresses: {
    resourceAddresses?: Record<string, string>;
    workflowAddresses?: Record<string, string>;
    secretReferences?: Record<string, string>;
  }
): Record<string, unknown> {
  return {
    artifact: params.artifact,
    deployPlan,
    functionName: params.input.functionName,
    startedAt: deploymentStartedAt,
    completedAt: new Date().toISOString(),
    resourceAddresses: addresses.resourceAddresses,
    workflowAddresses: addresses.workflowAddresses,
    secretReferences: addresses.secretReferences
  };
}

async function persistAppliedState(
  params: ProviderDeployInput,
  provider: ProviderAdapter,
  session: ProviderStateSession,
  deployment: Awaited<ReturnType<typeof runProviderDeployment>>
): Promise<void> {
  await params.context.stateBackend.write(
    session.stateAddress,
    {
      schemaVersion: 2,
      provider: provider.name,
      service: params.context.project.service,
      stage: params.context.stage,
      endpoint: deployment.endpoint,
      resourceAddresses: deployment.resourceAddresses,
      workflowAddresses: deployment.workflowAddresses,
      secretReferences: deployment.secretReferences,
      lifecycle: "applied",
      updatedAt: new Date().toISOString(),
      details: createAppliedStateDetails(params, deployment.deployPlan, session.deploymentStartedAt, {
        resourceAddresses: deployment.resourceAddresses,
        workflowAddresses: deployment.workflowAddresses,
        secretReferences: deployment.secretReferences
      })
    },
    session.heartbeat.latestLock()
  );
}

function createFailedStateDetails(
  params: ProviderDeployInput,
  deploymentStartedAt: string,
  message: string
): Record<string, unknown> {
  return {
    artifact: params.artifact,
    functionName: params.input.functionName,
    startedAt: deploymentStartedAt,
    failedAt: new Date().toISOString(),
    error: message
  };
}

async function persistFailedState(
  params: ProviderDeployInput,
  provider: ProviderAdapter,
  session: ProviderStateSession,
  message: string
): Promise<void> {
  await params.context.stateBackend.write(
    session.stateAddress,
    {
      schemaVersion: 2,
      provider: provider.name,
      service: params.context.project.service,
      stage: params.context.stage,
      lifecycle: "failed",
      updatedAt: new Date().toISOString(),
      details: createFailedStateDetails(params, session.deploymentStartedAt, message)
    },
    session.heartbeat.latestLock()
  );
}

async function closeProviderStateSession(
  stateBackend: StateBackend,
  session: ProviderStateSession,
  providerName: string,
  failures: DeployFailure[]
): Promise<void> {
  try {
    await session.heartbeat.stop();
    await stateBackend.unlock(session.stateAddress, session.heartbeat.latestLock());
  } catch (unlockError) {
    failures.push({
      provider: providerName,
      phase: "state",
      message: `failed to release state lock: ${stringifyError(unlockError)}`
    });
  }
}

export function createDeployCollections(): DeployCollections {
  return {
    deployments: [],
    failures: [],
    successfulDeployments: [],
    rollbacks: []
  };
}

function recordSuccessfulDeployment(
  collections: DeployCollections,
  provider: ProviderAdapter,
  deployment: Awaited<ReturnType<typeof runProviderDeployment>>
): void {
  collections.deployments.push({
    provider: provider.name,
    endpoint: deployment.endpoint,
    mode: deployment.mode
  });
  collections.successfulDeployments.push({ provider: provider.name, adapter: provider });
}

function recordFailedStatePersistence(
  collections: DeployCollections,
  providerName: string,
  stateError: unknown
): void {
  collections.failures.push({
    provider: providerName,
    phase: "state",
    message: `failed to persist failed state: ${stringifyError(stateError)}`
  });
}

export async function deploySingleArtifact(params: ProviderDeployInput): Promise<void> {
  const { context, collections, artifact } = params;
  const provider = context.providerRegistry[artifact.provider];

  if (!provider) {
    pushMissingAdapterFailure(artifact.provider, collections.failures);
    return;
  }

  logProgress(params.input.emitProgress, `${provider.name}: adapter loaded`);
  const valid = await validateProvider(provider, context.project, collections.failures);
  if (!valid) {
    return;
  }

  logProgress(params.input.emitProgress, `${provider.name}: validation passed`);
  const session = await openProviderStateSession(params, provider);
  logProgress(params.input.emitProgress, `${provider.name}: state -> in_progress`);

  try {
    logProgress(params.input.emitProgress, `${provider.name}: provisioning resources/workflows/secrets`);
    const deployment = await runProviderDeployment(params, provider);
    recordSuccessfulDeployment(collections, provider, deployment);

    await persistAppliedState(params, provider, session, deployment);
    logProgress(params.input.emitProgress, `${provider.name}: state -> applied`);
    logProgress(params.input.emitProgress, `${provider.name}: deploy completed`);
  } catch (deployError) {
    const message = stringifyError(deployError);
    collections.failures.push({ provider: provider.name, phase: "deploy", message });
    logProgress(params.input.emitProgress, `${provider.name}: deploy failed (${message})`);

    try {
      await persistFailedState(params, provider, session, message);
      logProgress(params.input.emitProgress, `${provider.name}: state -> failed`);
    } catch (stateError) {
      recordFailedStatePersistence(collections, provider.name, stateError);
    }
  } finally {
    await closeProviderStateSession(context.stateBackend, session, provider.name, collections.failures);
  }
}

function rollbackEnabled(): boolean {
  const configured = process.env.RUNFABRIC_ROLLBACK_ON_FAILURE;
  if (!configured) {
    return false;
  }
  const normalized = configured.trim().toLowerCase();
  return Boolean(normalized) && ["1", "true", "yes", "on"].includes(normalized);
}

export async function rollbackDeployments(
  input: DeployWorkflowInput,
  context: DeployContext,
  collections: DeployCollections
): Promise<void> {
  if (collections.failures.length === 0 || collections.successfulDeployments.length === 0) {
    return;
  }
  if (!rollbackEnabled()) {
    return;
  }

  logProgress(input.emitProgress, "deploy: rollback enabled, reverting successful providers");
  for (const deployedProvider of [...collections.successfulDeployments].reverse()) {
    const stateAddress = createStateAddress(context.project, context.stage, deployedProvider.provider);
    const cleanupTarget = resolve(input.projectDir, ".runfabric", "deploy", deployedProvider.provider);

    try {
      if (!deployedProvider.adapter.destroy) {
        throw new Error("provider destroy is not implemented");
      }

      await deployedProvider.adapter.destroy(context.project);
      await context.stateBackend.delete(stateAddress);
      await context.stateBackend.forceUnlock(stateAddress);
      await rm(cleanupTarget, { recursive: true, force: true });

      const deploymentIndex = collections.deployments.findIndex(
        (item) => item.provider === deployedProvider.provider
      );
      if (deploymentIndex >= 0) {
        collections.deployments.splice(deploymentIndex, 1);
      }

      collections.rollbacks.push({ provider: deployedProvider.provider, ok: true });
      logProgress(input.emitProgress, `${deployedProvider.provider}: rollback succeeded`);
    } catch (rollbackError) {
      const message = stringifyError(rollbackError);
      collections.rollbacks.push({ provider: deployedProvider.provider, ok: false, message });
      collections.failures.push({ provider: deployedProvider.provider, phase: "rollback", message });
      logProgress(input.emitProgress, `${deployedProvider.provider}: rollback failed (${message})`);
    }
  }
}
