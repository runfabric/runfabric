import type {
  BuildArtifact,
  BuildPlan,
  BuildResult,
  DeployPlan,
  DeployResult,
  ProjectConfig,
  ProviderAdapter,
  ProviderCredentialSchema,
  ValidationResult
} from "@runfabric/core";
import {
  appendProviderLog,
  buildProviderLogsFromLocalArtifacts,
  createProviderNativeObservabilityOperations,
  createStandardProviderPlanOperations,
  createDeploymentId,
  destroyProviderArtifacts,
  invokeProviderViaDeployedEndpoint,
  isRecordLike,
  isRealDeployModeEnabled,
  missingRequiredCredentialErrors,
  readNonEmptyString,
  runStandardCliRealDeployIfEnabled,
  runShellCommand,
  writeDeploymentReceipt
} from "@runfabric/core";
import { flyMachinesCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const flyCredentialSchema: ProviderCredentialSchema = {
  provider: "fly-machines",
  fields: [
    { env: "FLY_API_TOKEN", description: "Fly.io API token" },
    { env: "FLY_APP_NAME", description: "Fly app name" }
  ]
};

function toHttpEndpoint(value: string): string {
  if (value.startsWith("http://") || value.startsWith("https://")) {
    return value;
  }
  return `https://${value}`;
}

function endpointFromFlyResponse(response: unknown): string | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const direct =
    readNonEmptyString(response.endpoint) ||
    readNonEmptyString(response.url) ||
    readNonEmptyString(response.hostname);
  if (direct) {
    return toHttpEndpoint(direct);
  }

  if (!isRecordLike(response.app)) {
    return undefined;
  }

  const host = readNonEmptyString(response.app.hostname) || readNonEmptyString(response.app.name);
  if (!host) {
    return undefined;
  }
  return host.includes(".") ? `https://${host}` : `https://${host}.fly.dev`;
}

function flyResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["id", "name", "region", "status"]) {
    const value = readNonEmptyString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultFlyDeployCommand(appName: string): string {
  return ["flyctl deploy", "--json", "--remote-only", `--app ${JSON.stringify(appName)}`].join(" ");
}

function defaultFlyDestroyCommand(appName: string): string {
  return `flyctl apps destroy ${JSON.stringify(appName)} --yes`;
}

function resolveAppName(project: ProjectConfig): string {
  const extension = project.extensions?.["fly-machines"];
  if (typeof extension?.appName === "string" && extension.appName.trim().length > 0) {
    return extension.appName;
  }
  return process.env.FLY_APP_NAME || project.service;
}

async function deployFlyMachines(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const appName = resolveAppName(project);
  const deploymentId = createDeploymentId("fly-machines", appName, stage);
  const deployState = await runStandardCliRealDeployIfEnabled({
    projectDir: options.projectDir,
    project,
    plan,
    stage,
    realDeployEnv: "RUNFABRIC_FLY_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_FLY_DEPLOY_CMD",
    defaultDeployCommand: defaultFlyDeployCommand(appName),
    defaultEndpoint: `https://${appName}.fly.dev`,
    parseEndpoint: endpointFromFlyResponse,
    missingEndpointError: "fly-machines deploy response does not include endpoint",
    buildResource: (rawResponse) => flyResourceMetadata(rawResponse),
    extraResource: { appName }
  });

  await writeDeploymentReceipt(options.projectDir, "fly-machines", {
    provider: "fly-machines",
    service: project.service,
    stage,
    deploymentId,
    endpoint: deployState.endpoint,
    mode: deployState.mode,
    executedSteps: plan.steps,
    artifactPath: plan.artifactPath,
    artifactManifestPath: plan.artifactManifestPath,
    resource: deployState.resource,
    rawResponse: deployState.rawResponse,
    createdAt: new Date().toISOString()
  });
  await appendProviderLog(
    options.projectDir,
    "fly-machines",
    `deploy deploymentId=${deploymentId} mode=${deployState.mode} endpoint=${deployState.endpoint}`
  );

  return { provider: "fly-machines", endpoint: deployState.endpoint };
}

async function destroyFlyMachines(options: ProviderOptions, project: ProjectConfig): Promise<void> {
  if (isRealDeployModeEnabled("RUNFABRIC_FLY_REAL_DEPLOY")) {
    const stage = project.stage || "default";
    const destroyCommand = process.env.RUNFABRIC_FLY_DESTROY_CMD || defaultFlyDestroyCommand(resolveAppName(project));
    const result = await runShellCommand(destroyCommand, {
      cwd: options.projectDir,
      env: {
        RUNFABRIC_SERVICE: project.service,
        RUNFABRIC_STAGE: stage
      }
    });
    if (result.code !== 0) {
      throw new Error(result.stderr || result.stdout || "fly-machines destroy command failed");
    }
  }

  await appendProviderLog(options.projectDir, "fly-machines", "destroy local artifacts");
  await destroyProviderArtifacts(options.projectDir, "fly-machines");
}

function validateFlyProvider(): ValidationResult {
  const errors = missingRequiredCredentialErrors(flyCredentialSchema);
  return { ok: errors.length === 0, warnings: [], errors };
}

const flyPlanOperations = createStandardProviderPlanOperations(
  "fly-machines",
  "prepare fly machines metadata"
);

export function createFlyMachinesProvider(options: ProviderOptions): ProviderAdapter {
  const observabilityOperations = createProviderNativeObservabilityOperations({
    projectDir: options.projectDir,
    provider: "fly-machines",
    realDeployEnv: "RUNFABRIC_FLY_REAL_DEPLOY",
    tracesCommandEnv: "RUNFABRIC_FLY_TRACES_CMD",
    metricsCommandEnv: "RUNFABRIC_FLY_METRICS_CMD"
  });

  return {
    name: "fly-machines",
    getCapabilities: () => flyMachinesCapabilities,
    getCredentialSchema: () => flyCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateFlyProvider(),
    planBuild: flyPlanOperations.planBuild,
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: flyPlanOperations.planDeploy,
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployFlyMachines(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "fly-machines", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "fly-machines", input),
    traces: observabilityOperations.traces,
    metrics: observabilityOperations.metrics,
    destroy: async (project: ProjectConfig) => destroyFlyMachines(options, project)
  };
}
