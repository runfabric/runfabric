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

function toHttpEndpoint(value: string): string {
  return value.startsWith("http") ? value : `https://${value}`;
}

function endpointFromIbmResponse(response: unknown): string | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const endpoint =
    readNonEmptyString(response.endpoint) ||
    readNonEmptyString(response.url) ||
    readNonEmptyString(response.apihost);
  if (endpoint) {
    return toHttpEndpoint(endpoint);
  }

  if (!isRecordLike(response.result)) {
    return undefined;
  }
  const nested =
    readNonEmptyString(response.result.url) ||
    readNonEmptyString(response.result.endpoint);
  return nested ? toHttpEndpoint(nested) : undefined;
}

function ibmResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["namespace", "name", "version"]) {
    const value = readNonEmptyString(response[key]);
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

async function deployIbmOpenWhisk(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const namespace = resolveNamespace(project);
  const deploymentId = createDeploymentId("ibm-openwhisk", project.service, stage);
  const deployState = await runStandardCliRealDeployIfEnabled({
    projectDir: options.projectDir,
    project,
    plan,
    stage,
    realDeployEnv: "RUNFABRIC_IBM_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_IBM_DEPLOY_CMD",
    defaultDeployCommand: defaultIbmDeployCommand(),
    defaultEndpoint: defaultEndpoint(project.service, namespace),
    parseEndpoint: endpointFromIbmResponse,
    missingEndpointError: "ibm-openwhisk deploy response does not include endpoint",
    buildResource: (rawResponse) => ibmResourceMetadata(rawResponse),
    extraResource: {
      region: process.env.IBM_CLOUD_REGION || "us-south",
      namespace
    }
  });

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

const ibmPlanOperations = createStandardProviderPlanOperations(
  "ibm-openwhisk",
  "prepare ibm openwhisk metadata"
);

export function createIbmOpenWhiskProvider(options: ProviderOptions): ProviderAdapter {
  const observabilityOperations = createProviderNativeObservabilityOperations({
    projectDir: options.projectDir,
    provider: "ibm-openwhisk",
    realDeployEnv: "RUNFABRIC_IBM_REAL_DEPLOY",
    tracesCommandEnv: "RUNFABRIC_IBM_TRACES_CMD",
    metricsCommandEnv: "RUNFABRIC_IBM_METRICS_CMD"
  });

  return {
    name: "ibm-openwhisk",
    getCapabilities: () => ibmOpenWhiskCapabilities,
    getCredentialSchema: () => ibmCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateIbmProvider(),
    planBuild: ibmPlanOperations.planBuild,
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: ibmPlanOperations.planDeploy,
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployIbmOpenWhisk(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "ibm-openwhisk", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "ibm-openwhisk", input),
    traces: observabilityOperations.traces,
    metrics: observabilityOperations.metrics,
    destroy: async (project: ProjectConfig) => destroyIbmOpenWhisk(options, project)
  };
}
