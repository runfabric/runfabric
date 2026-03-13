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
import { vercelCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const vercelCredentialSchema: ProviderCredentialSchema = {
  provider: "vercel",
  fields: [
    { env: "VERCEL_TOKEN", description: "Vercel API token" },
    { env: "VERCEL_ORG_ID", description: "Vercel team or user ID" },
    { env: "VERCEL_PROJECT_ID", description: "Vercel project ID" }
  ]
};

function toHttpEndpoint(endpoint: string): string {
  if (endpoint.startsWith("http://") || endpoint.startsWith("https://")) {
    return endpoint;
  }
  return `https://${endpoint}`;
}

function endpointFromVercelResponse(response: unknown): string | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const direct =
    readNonEmptyString(response.endpoint) ||
    readNonEmptyString(response.url) ||
    readNonEmptyString(response.alias) ||
    readNonEmptyString(response.inspectorUrl);
  if (direct) {
    return toHttpEndpoint(direct);
  }

  if (Array.isArray(response.alias) && response.alias.length > 0) {
    const alias = readNonEmptyString(response.alias[0]);
    return alias ? toHttpEndpoint(alias) : undefined;
  }

  return undefined;
}

function vercelResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["id", "name", "projectId", "readyState"]) {
    const value = readNonEmptyString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultVercelDeployCommand(): string {
  return 'vercel deploy --yes --prod --json --cwd "${RUNFABRIC_VERCEL_SOURCE_DIR:-.}"';
}

function defaultVercelDestroyCommand(): string {
  return 'vercel remove "$RUNFABRIC_SERVICE" --yes';
}

function resolveProjectName(project: ProjectConfig): string {
  const extension = project.extensions?.vercel;
  if (typeof extension?.projectName === "string" && extension.projectName.trim().length > 0) {
    return extension.projectName;
  }
  return project.service;
}

async function deployVercel(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const projectName = resolveProjectName(project);
  const deploymentId = createDeploymentId("vercel", projectName, stage);
  const deployState = await runStandardCliRealDeployIfEnabled({
    projectDir: options.projectDir,
    project,
    plan,
    stage,
    realDeployEnv: "RUNFABRIC_VERCEL_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_VERCEL_DEPLOY_CMD",
    defaultDeployCommand: defaultVercelDeployCommand(),
    defaultEndpoint: `https://${projectName}.vercel.app`,
    parseEndpoint: endpointFromVercelResponse,
    missingEndpointError: "vercel deploy response does not include deployment URL",
    buildResource: (rawResponse) => vercelResourceMetadata(rawResponse),
    extraResource: { projectName }
  });

  await writeDeploymentReceipt(options.projectDir, "vercel", {
    provider: "vercel",
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
    "vercel",
    `deploy deploymentId=${deploymentId} mode=${deployState.mode} endpoint=${deployState.endpoint}`
  );

  return { provider: "vercel", endpoint: deployState.endpoint };
}

async function destroyVercel(options: ProviderOptions, project: ProjectConfig): Promise<void> {
  if (isRealDeployModeEnabled("RUNFABRIC_VERCEL_REAL_DEPLOY")) {
    const stage = project.stage || "default";
    const destroyCommand = process.env.RUNFABRIC_VERCEL_DESTROY_CMD || defaultVercelDestroyCommand();
    const result = await runShellCommand(destroyCommand, {
      cwd: options.projectDir,
      env: {
        RUNFABRIC_SERVICE: project.service,
        RUNFABRIC_STAGE: stage
      }
    });
    if (result.code !== 0) {
      throw new Error(result.stderr || result.stdout || "vercel destroy command failed");
    }
  }

  await appendProviderLog(options.projectDir, "vercel", "destroy local artifacts");
  await destroyProviderArtifacts(options.projectDir, "vercel");
}

function validateVercelProvider(): ValidationResult {
  const errors = missingRequiredCredentialErrors(vercelCredentialSchema);
  return { ok: errors.length === 0, warnings: [], errors };
}

const vercelPlanOperations = createStandardProviderPlanOperations(
  "vercel",
  "prepare vercel function metadata"
);

export function createVercelProvider(options: ProviderOptions): ProviderAdapter {
  const observabilityOperations = createProviderNativeObservabilityOperations({
    projectDir: options.projectDir,
    provider: "vercel",
    realDeployEnv: "RUNFABRIC_VERCEL_REAL_DEPLOY",
    tracesCommandEnv: "RUNFABRIC_VERCEL_TRACES_CMD",
    metricsCommandEnv: "RUNFABRIC_VERCEL_METRICS_CMD"
  });

  return {
    name: "vercel",
    getCapabilities: () => vercelCapabilities,
    getCredentialSchema: () => vercelCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateVercelProvider(),
    planBuild: vercelPlanOperations.planBuild,
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: vercelPlanOperations.planDeploy,
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployVercel(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "vercel", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "vercel", input),
    traces: observabilityOperations.traces,
    metrics: observabilityOperations.metrics,
    destroy: async (project: ProjectConfig) => destroyVercel(options, project)
  };
}
