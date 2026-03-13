import type { CommandRegistrar } from "../types/cli";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { createStateBackend, type StateAddress } from "@runfabric/core";
import { createProviderRegistry, getProviderPackageName } from "../providers/registry";
import { loadPlanningContext } from "../utils/load-config";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info, warn } from "../utils/logger";

interface RemoveFailure {
  provider: string;
  message: string;
}

interface RemoveRecoveryArtifact {
  provider: string;
  path: string;
}

interface RemoveOptions {
  config?: string;
  stage?: string;
  provider?: string;
  json?: boolean;
}

export interface RemoveWorkflowInput {
  projectDir: string;
  configPath?: string;
  stage?: string;
  provider?: string;
}

export interface RemoveWorkflowResult {
  service: string;
  stage: string;
  providers: string[];
  destroyedProviders: string[];
  removedPaths: string[];
  failures: RemoveFailure[];
  recoveryArtifacts: RemoveRecoveryArtifact[];
  summary: {
    exitCode: number;
  };
}

interface RemoveCollections {
  removedPaths: string[];
  destroyedProviders: string[];
  failures: RemoveFailure[];
  recoveryArtifacts: RemoveRecoveryArtifact[];
}

function createCollections(): RemoveCollections {
  return {
    removedPaths: [],
    destroyedProviders: [],
    failures: [],
    recoveryArtifacts: []
  };
}

function toErrorMessage(errorValue: unknown): string {
  return errorValue instanceof Error ? errorValue.message : String(errorValue);
}

async function writeRecoveryArtifact(
  recoveryDir: string,
  providerName: string,
  payload: Record<string, unknown>,
  collections: RemoveCollections
): Promise<void> {
  const recoveryPath = resolve(
    recoveryDir,
    `${providerName}.${new Date().toISOString().replace(/[:.]/g, "-")}.json`
  );

  await writeFile(recoveryPath, JSON.stringify(payload, null, 2), "utf8");
  collections.recoveryArtifacts.push({ provider: providerName, path: recoveryPath });
}

function resolveProviderTargets(projectProviders: string[], providerOption?: string): string[] {
  return providerOption ? [providerOption] : projectProviders;
}

function recordMissingProviderFailure(providerName: string, collections: RemoveCollections): void {
  const packageName = getProviderPackageName(providerName);
  collections.failures.push({
    provider: providerName,
    message: packageName
      ? `provider adapter is not installed (${packageName})`
      : "provider adapter is not installed"
  });
}

async function runProviderDestroy(
  providerName: string,
  provider: NonNullable<ReturnType<typeof createProviderRegistry>[string]>,
  project: Awaited<ReturnType<typeof loadPlanningContext>>["project"],
  collections: RemoveCollections,
  recoveryDir: string,
  stage: string
): Promise<boolean> {
  if (!provider.destroy) {
    return true;
  }

  try {
    await provider.destroy(project);
    collections.destroyedProviders.push(providerName);
    return true;
  } catch (destroyError) {
    const message = `provider destroy failed: ${toErrorMessage(destroyError)}`;
    collections.failures.push({ provider: providerName, message });

    await writeRecoveryArtifact(
      recoveryDir,
      providerName,
      {
        provider: providerName,
        service: project.service,
        stage,
        message,
        createdAt: new Date().toISOString(),
        suggestion: "re-run runfabric remove after fixing provider credentials/permissions"
      },
      collections
    );

    return false;
  }
}

async function cleanupProviderState(
  projectService: string,
  stage: string,
  providerName: string,
  stateBackend: ReturnType<typeof createStateBackend>,
  collections: RemoveCollections
): Promise<void> {
  const stateAddress: StateAddress = {
    service: projectService,
    stage,
    provider: providerName
  };

  try {
    await stateBackend.delete(stateAddress);
    await stateBackend.forceUnlock(stateAddress);
  } catch (stateError) {
    collections.failures.push({
      provider: providerName,
      message: `state cleanup failed: ${toErrorMessage(stateError)}`
    });
  }
}

function providerCleanupPaths(projectDir: string, providerName: string, service: string): string[] {
  return [
    resolve(projectDir, ".runfabric", "deploy", providerName),
    resolve(projectDir, ".runfabric", "build", providerName, service)
  ];
}

async function cleanupProviderPaths(
  paths: string[],
  providerName: string,
  projectService: string,
  stage: string,
  recoveryDir: string,
  collections: RemoveCollections
): Promise<void> {
  for (const targetPath of paths) {
    try {
      await rm(targetPath, { recursive: true, force: true });
      collections.removedPaths.push(targetPath);
    } catch (removeError) {
      collections.failures.push({
        provider: providerName,
        message: `failed to remove ${targetPath}: ${toErrorMessage(removeError)}`
      });

      await writeRecoveryArtifact(
        recoveryDir,
        providerName,
        {
          provider: providerName,
          service: projectService,
          stage,
          failedPath: targetPath,
          createdAt: new Date().toISOString(),
          suggestion: "remove the path manually or re-run runfabric remove"
        },
        collections
      );
    }
  }
}

function printRemoveResult(
  payload: {
    service: string;
    stage: string;
    providers: string[];
    destroyedProviders: string[];
    removedPaths: string[];
    failures: RemoveFailure[];
    recoveryArtifacts: RemoveRecoveryArtifact[];
  },
  json: boolean | undefined
): void {
  if (json) {
    printJson(payload);
    return;
  }

  info(`remove completed for service ${payload.service} (${payload.stage})`);
  if (payload.destroyedProviders.length > 0) {
    info(`provider destroy executed: ${payload.destroyedProviders.join(", ")}`);
  }
  info(`removed paths: ${payload.removedPaths.length}`);
  info(`recovery artifacts: ${payload.recoveryArtifacts.length}`);
  if (payload.failures.length > 0) {
    warn(`failures: ${payload.failures.length}`);
    for (const failure of payload.failures) {
      error(`${failure.provider}: ${failure.message}`);
    }
  }
}

export async function executeRemoveWorkflow(input: RemoveWorkflowInput): Promise<RemoveWorkflowResult> {
  const context = await loadPlanningContext(input.projectDir, input.configPath, input.stage);
  const stage = context.project.stage || "default";
  const providers = resolveProviderTargets(context.project.providers, input.provider);
  const registry = createProviderRegistry(input.projectDir, providers);
  const stateBackend = createStateBackend({ projectDir: input.projectDir, state: context.project.state });

  const collections = createCollections();
  const recoveryDir = resolve(input.projectDir, ".runfabric", "recovery", "remove");
  await mkdir(recoveryDir, { recursive: true });

  await processProviderRemovals({
    providers,
    registry,
    context,
    stage,
    projectDir: input.projectDir,
    stateBackend,
    recoveryDir,
    collections
  });

  return {
    service: context.project.service,
    stage,
    providers,
    destroyedProviders: collections.destroyedProviders,
    removedPaths: collections.removedPaths,
    failures: collections.failures,
    recoveryArtifacts: collections.recoveryArtifacts,
    summary: {
      exitCode: collections.failures.length > 0 ? 1 : 0
    }
  };
}

async function executeRemoveCommand(options: RemoveOptions): Promise<void> {
  const projectDir = await resolveProjectDir(process.cwd(), options.config);
  const result = await executeRemoveWorkflow({
    projectDir,
    configPath: options.config ? resolve(process.cwd(), options.config) : undefined,
    stage: options.stage,
    provider: options.provider
  });

  if (result.summary.exitCode !== 0) {
    process.exitCode = result.summary.exitCode;
  }

  printRemoveResult(result, options.json);
}

async function processProviderRemovals(input: {
  providers: string[];
  registry: ReturnType<typeof createProviderRegistry>;
  context: Awaited<ReturnType<typeof loadPlanningContext>>;
  stage: string;
  projectDir: string;
  stateBackend: ReturnType<typeof createStateBackend>;
  recoveryDir: string;
  collections: RemoveCollections;
}): Promise<void> {
  const { providers, registry, context, stage, projectDir, stateBackend, recoveryDir, collections } = input;
  for (const providerName of providers) {
    const provider = registry[providerName];
    if (!provider) {
      recordMissingProviderFailure(providerName, collections);
      continue;
    }

    const destroyOk = await runProviderDestroy(
      providerName,
      provider,
      context.project,
      collections,
      recoveryDir,
      stage
    );
    if (!destroyOk) {
      continue;
    }

    await cleanupProviderState(context.project.service, stage, providerName, stateBackend, collections);
    await cleanupProviderPaths(
      providerCleanupPaths(projectDir, providerName, context.project.service),
      providerName,
      context.project.service,
      stage,
      recoveryDir,
      collections
    );
  }
}

export const registerRemoveCommand: CommandRegistrar = (program) => {
  program
    .command("remove")
    .description("Remove deployed artifacts/state and invoke provider cleanup flows")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .option("-p, --provider <name>", "Limit removal to a single provider")
    .option("--json", "Emit JSON output")
    .action(async (options: RemoveOptions) => executeRemoveCommand(options));
};
