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
import { ibmOpenWhiskCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const ibmCredentialSchema: ProviderCredentialSchema = {
  provider: "ibm-openwhisk",
  fields: [
    { env: "IBM_CLOUD_API_KEY", description: "IBM Cloud API key" },
    { env: "IBM_CLOUD_REGION", description: "IBM Cloud region" },
    { env: "IBM_CLOUD_NAMESPACE", description: "IBM Cloud Functions namespace" }
  ]
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function toHttpEndpoint(value: string): string {
  return value.startsWith("http") ? value : `https://${value}`;
}

function endpointFromIbmResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const endpoint = readString(response.endpoint) || readString(response.url) || readString(response.apihost);
  if (endpoint) {
    return toHttpEndpoint(endpoint);
  }

  if (!isRecord(response.result)) {
    return undefined;
  }
  const nested = readString(response.result.url) || readString(response.result.endpoint);
  return nested ? toHttpEndpoint(nested) : undefined;
}

function ibmResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["namespace", "name", "version"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultIbmDeployCommand(): string {
  return [
    'ibmcloud fn action update "$RUNFABRIC_SERVICE" "$RUNFABRIC_ARTIFACT_PATH"',
    '--kind "${RUNFABRIC_IBM_RUNTIME_KIND:-nodejs:20}"',
    "--web true",
    "--result",
    "--output json"
  ].join(" ");
}

function defaultIbmDestroyCommand(): string {
  return 'ibmcloud fn action delete "$RUNFABRIC_SERVICE"';
}

function resolveNamespace(project: ProjectConfig): string {
  const extension = project.extensions?.["ibm-openwhisk"];
  if (typeof extension?.namespace === "string" && extension.namespace.trim().length > 0) {
    return extension.namespace;
  }
  return process.env.IBM_CLOUD_NAMESPACE || "default";
}

function defaultEndpoint(service: string, namespace: string): string {
  const region = process.env.IBM_CLOUD_REGION || "us-south";
  return `https://${region}.functions.cloud.ibm.com/api/v1/web/${namespace}/default/${service}`;
}

async function runRealDeployIfEnabled(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan,
  stage: string,
  namespace: string
): Promise<{
  endpoint: string;
  mode: "simulated" | "cli";
  rawResponse?: unknown;
  resource?: Record<string, unknown>;
}> {
  const initialEndpoint = defaultEndpoint(project.service, namespace);
  if (!isRealDeployModeEnabled("RUNFABRIC_IBM_REAL_DEPLOY")) {
    return { endpoint: initialEndpoint, mode: "simulated" };
  }

  const region = process.env.IBM_CLOUD_REGION || "us-south";
  const deployCommand = process.env.RUNFABRIC_IBM_DEPLOY_CMD || defaultIbmDeployCommand();
  const hasCommandOverride = Boolean(process.env.RUNFABRIC_IBM_DEPLOY_CMD);
  const rawResponse = await runJsonCommand(deployCommand, {
    cwd: options.projectDir,
    env: {
      RUNFABRIC_SERVICE: project.service,
      RUNFABRIC_STAGE: stage,
      RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
      RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath
    }
  });

  const parsedEndpoint = endpointFromIbmResponse(rawResponse);
  if (!parsedEndpoint && hasCommandOverride) {
    throw new Error("ibm-openwhisk deploy response does not include endpoint");
  }

  return {
    endpoint: parsedEndpoint || initialEndpoint,
    mode: "cli",
    rawResponse,
    resource: {
      ...(ibmResourceMetadata(rawResponse) || {}),
      region,
      namespace,
      deployCommandSource: hasCommandOverride ? "override" : "builtin"
    }
  };
}

async function deployIbmOpenWhisk(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const namespace = resolveNamespace(project);
  const deploymentId = createDeploymentId("ibm-openwhisk", project.service, stage);
  const deployState = await runRealDeployIfEnabled(options, project, plan, stage, namespace);

  await writeDeploymentReceipt(options.projectDir, "ibm-openwhisk", {
    provider: "ibm-openwhisk",
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
    "ibm-openwhisk",
    `deploy deploymentId=${deploymentId} mode=${deployState.mode} endpoint=${deployState.endpoint}`
  );

  return { provider: "ibm-openwhisk", endpoint: deployState.endpoint };
}

async function destroyIbmOpenWhisk(options: ProviderOptions, project: ProjectConfig): Promise<void> {
  if (isRealDeployModeEnabled("RUNFABRIC_IBM_REAL_DEPLOY")) {
    const stage = project.stage || "default";
    const destroyCommand = process.env.RUNFABRIC_IBM_DESTROY_CMD || defaultIbmDestroyCommand();
    const result = await runShellCommand(destroyCommand, {
      cwd: options.projectDir,
      env: {
        RUNFABRIC_SERVICE: project.service,
        RUNFABRIC_STAGE: stage
      }
    });
    if (result.code !== 0) {
      throw new Error(result.stderr || result.stdout || "ibm-openwhisk destroy command failed");
    }
  }

  await appendProviderLog(options.projectDir, "ibm-openwhisk", "destroy local artifacts");
  await destroyProviderArtifacts(options.projectDir, "ibm-openwhisk");
}

function validateIbmProvider(): ValidationResult {
  const errors = missingRequiredCredentialErrors(ibmCredentialSchema);
  return { ok: errors.length === 0, warnings: [], errors };
}

function createBuildPlan(): BuildPlan {
  return {
    provider: "ibm-openwhisk",
    steps: ["prepare ibm openwhisk metadata"]
  };
}

function createDeployPlan(artifact: BuildArtifact): DeployPlan {
  return {
    provider: "ibm-openwhisk",
    artifactPath: artifact.entry,
    artifactManifestPath: artifact.outputPath,
    steps: [`deploy artifact from ${artifact.outputPath}`]
  };
}

export function createIbmOpenWhiskProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "ibm-openwhisk",
    getCapabilities: () => ibmOpenWhiskCapabilities,
    getCredentialSchema: () => ibmCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateIbmProvider(),
    planBuild: async (): Promise<BuildPlan> => createBuildPlan(),
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: async (_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> =>
      createDeployPlan(artifact),
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployIbmOpenWhisk(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "ibm-openwhisk", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "ibm-openwhisk", input),
    destroy: async (project: ProjectConfig) => destroyIbmOpenWhisk(options, project)
  };
}
