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
  createDeploymentId,
  destroyProviderArtifacts,
  invokeProviderViaDeployedEndpoint,
  isRealDeployModeEnabled,
  missingRequiredCredentialErrors,
  runJsonCommand,
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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function endpointFromGcpResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const directCandidates = [response.endpoint, response.url, response.uri];
  for (const candidate of directCandidates) {
    const endpoint = readString(candidate);
    if (endpoint) {
      return endpoint;
    }
  }

  if (isRecord(response.httpsTrigger)) {
    const endpoint = readString(response.httpsTrigger.url);
    if (endpoint) {
      return endpoint;
    }
  }

  if (isRecord(response.serviceConfig)) {
    const endpoint = readString(response.serviceConfig.uri);
    if (endpoint) {
      return endpoint;
    }
  }

  return undefined;
}

function gcpResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["name", "state", "updateTime", "buildId"]) {
    const value = readString(response[key]);
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

async function runRealDeployIfEnabled(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan,
  region: string,
  stage: string,
  currentEndpoint: string
): Promise<{
  endpoint: string;
  mode: "simulated" | "cli";
  rawResponse?: unknown;
  resource?: Record<string, unknown>;
}> {
  if (!isRealDeployModeEnabled("RUNFABRIC_GCP_REAL_DEPLOY")) {
    return { endpoint: currentEndpoint, mode: "simulated" };
  }

  const deployCommand = process.env.RUNFABRIC_GCP_DEPLOY_CMD || defaultGcpDeployCommand(region);
  const hasCommandOverride = Boolean(process.env.RUNFABRIC_GCP_DEPLOY_CMD);
  const rawResponse = await runJsonCommand(deployCommand, {
    cwd: options.projectDir,
    env: {
      RUNFABRIC_SERVICE: project.service,
      RUNFABRIC_STAGE: stage,
      RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
      RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath
    }
  });

  const parsedEndpoint = endpointFromGcpResponse(rawResponse);
  if (!parsedEndpoint && hasCommandOverride) {
    throw new Error("gcp-functions deploy response does not include endpoint URL");
  }

  return {
    endpoint: parsedEndpoint || currentEndpoint,
    mode: "cli",
    rawResponse,
    resource: {
      ...(gcpResourceMetadata(rawResponse) || {}),
      deployCommandSource: hasCommandOverride ? "override" : "builtin"
    }
  };
}

async function deployGcpFunctions(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const region = resolveRegion(project);
  const deploymentId = createDeploymentId("gcp-functions", project.service, stage);

  const deployState = await runRealDeployIfEnabled(
    options,
    project,
    plan,
    region,
    stage,
    defaultEndpoint(project.service, region)
  );

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

function createPlanBuildResult(): BuildPlan {
  return {
    provider: "gcp-functions",
    steps: ["prepare gcp function metadata"]
  };
}

function createPlanDeployResult(artifact: BuildArtifact): DeployPlan {
  return {
    provider: "gcp-functions",
    artifactPath: artifact.entry,
    artifactManifestPath: artifact.outputPath,
    steps: [`deploy artifact from ${artifact.outputPath}`]
  };
}

export function createGcpFunctionsProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "gcp-functions",
    getCapabilities: () => gcpFunctionsCapabilities,
    getCredentialSchema: () => gcpCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateGcpProvider(),
    planBuild: async (): Promise<BuildPlan> => createPlanBuildResult(),
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: async (_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> =>
      createPlanDeployResult(artifact),
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployGcpFunctions(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "gcp-functions", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "gcp-functions", input),
    destroy: async (project: ProjectConfig) => destroyGcpFunctions(options, project)
  };
}
