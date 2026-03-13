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
import { gcpFunctionsCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const gcpCredentialSchema: ProviderCredentialSchema = {
  provider: "gcp-functions",
  fields: [
    { env: "GCP_PROJECT_ID", description: "Google Cloud project ID" },
    {
      env: "GCP_SERVICE_ACCOUNT_KEY",
      description: "Google Cloud service account JSON key (raw JSON or base64-decoded content)"
    }
  ]
};

function endpointFromGcpResponse(response: unknown): string | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const directCandidates = [response.endpoint, response.url, response.uri];
  for (const candidate of directCandidates) {
    const endpoint = readNonEmptyString(candidate);
    if (endpoint) {
      return endpoint;
    }
  }

  if (isRecordLike(response.httpsTrigger)) {
    const endpoint = readNonEmptyString(response.httpsTrigger.url);
    if (endpoint) {
      return endpoint;
    }
  }

  if (isRecordLike(response.serviceConfig)) {
    const endpoint = readNonEmptyString(response.serviceConfig.uri);
    if (endpoint) {
      return endpoint;
    }
  }

  return undefined;
}

function gcpResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["name", "state", "updateTime", "buildId"]) {
    const value = readNonEmptyString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultGcpDeployCommand(region: string): string {
  return [
    'gcloud functions deploy "$RUNFABRIC_SERVICE"',
    "--gen2",
    `--region=${JSON.stringify(region)}`,
    '--runtime="${RUNFABRIC_GCP_RUNTIME:-nodejs20}"',
    '--entry-point="${RUNFABRIC_GCP_ENTRY_POINT:-handler}"',
    "--trigger-http",
    "--allow-unauthenticated",
    '--source="${RUNFABRIC_GCP_SOURCE_DIR:-.}"',
    "--format=json"
  ].join(" ");
}

function defaultGcpDestroyCommand(region: string): string {
  return [
    'gcloud functions delete "$RUNFABRIC_SERVICE"',
    "--gen2",
    `--region=${JSON.stringify(region)}`,
    "--quiet",
    "--format=json"
  ].join(" ");
}

function resolveRegion(project: ProjectConfig): string {
  const extension = project.extensions?.["gcp-functions"];
  return typeof extension?.region === "string" ? extension.region : process.env.GCP_REGION || "us-central1";
}

function defaultEndpoint(service: string, region: string): string {
  const projectId = process.env.GCP_PROJECT_ID || "project-id";
  return `https://${region}-${projectId}.cloudfunctions.net/${service}`;
}

async function deployGcpFunctions(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const region = resolveRegion(project);
  const deploymentId = createDeploymentId("gcp-functions", project.service, stage);
  const deployState = await runStandardCliRealDeployIfEnabled({
    projectDir: options.projectDir,
    project,
    plan,
    stage,
    realDeployEnv: "RUNFABRIC_GCP_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_GCP_DEPLOY_CMD",
    defaultDeployCommand: defaultGcpDeployCommand(region),
    defaultEndpoint: defaultEndpoint(project.service, region),
    parseEndpoint: endpointFromGcpResponse,
    missingEndpointError: "gcp-functions deploy response does not include endpoint URL",
    buildResource: (rawResponse) => gcpResourceMetadata(rawResponse)
  });

  await writeDeploymentReceipt(options.projectDir, "gcp-functions", {
    provider: "gcp-functions",
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
    "gcp-functions",
    `deploy deploymentId=${deploymentId} mode=${deployState.mode} endpoint=${deployState.endpoint}`
  );

  return { provider: "gcp-functions", endpoint: deployState.endpoint };
}

async function destroyGcpFunctions(options: ProviderOptions, project: ProjectConfig): Promise<void> {
  if (isRealDeployModeEnabled("RUNFABRIC_GCP_REAL_DEPLOY")) {
    const stage = project.stage || "default";
    const destroyCommand = process.env.RUNFABRIC_GCP_DESTROY_CMD || defaultGcpDestroyCommand(resolveRegion(project));
    const result = await runShellCommand(destroyCommand, {
      cwd: options.projectDir,
      env: {
        RUNFABRIC_SERVICE: project.service,
        RUNFABRIC_STAGE: stage
      }
    });
    if (result.code !== 0) {
      throw new Error(result.stderr || result.stdout || "gcp-functions destroy command failed");
    }
  }

  await appendProviderLog(options.projectDir, "gcp-functions", "destroy local artifacts");
  await destroyProviderArtifacts(options.projectDir, "gcp-functions");
}

function validateGcpProvider(): ValidationResult {
  const errors = missingRequiredCredentialErrors(gcpCredentialSchema);
  return { ok: errors.length === 0, warnings: [], errors };
}

const gcpPlanOperations = createStandardProviderPlanOperations(
  "gcp-functions",
  "prepare gcp function metadata"
);

export function createGcpFunctionsProvider(options: ProviderOptions): ProviderAdapter {
  const observabilityOperations = createProviderNativeObservabilityOperations({
    projectDir: options.projectDir,
    provider: "gcp-functions",
    realDeployEnv: "RUNFABRIC_GCP_REAL_DEPLOY",
    tracesCommandEnv: "RUNFABRIC_GCP_TRACES_CMD",
    metricsCommandEnv: "RUNFABRIC_GCP_METRICS_CMD"
  });

  return {
    name: "gcp-functions",
    getCapabilities: () => gcpFunctionsCapabilities,
    getCredentialSchema: () => gcpCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateGcpProvider(),
    planBuild: gcpPlanOperations.planBuild,
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: gcpPlanOperations.planDeploy,
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployGcpFunctions(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "gcp-functions", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "gcp-functions", input),
    traces: observabilityOperations.traces,
    metrics: observabilityOperations.metrics,
    destroy: async (project: ProjectConfig) => destroyGcpFunctions(options, project)
  };
}
