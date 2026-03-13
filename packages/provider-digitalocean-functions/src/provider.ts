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
import { digitalOceanFunctionsCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const digitalOceanCredentialSchema: ProviderCredentialSchema = {
  provider: "digitalocean-functions",
  fields: [
    { env: "DIGITALOCEAN_ACCESS_TOKEN", description: "DigitalOcean API token" },
    { env: "DIGITALOCEAN_NAMESPACE", description: "DigitalOcean Functions namespace" }
  ]
};

function endpointFromDoResponse(response: unknown): string | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const endpoint =
    readNonEmptyString(response.endpoint) ||
    readNonEmptyString(response.url) ||
    readNonEmptyString(response.webUrl);
  if (endpoint) {
    return endpoint;
  }

  if (isRecordLike(response.result)) {
    return readNonEmptyString(response.result.url) || readNonEmptyString(response.result.endpoint);
  }

  return undefined;
}

function doResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["name", "namespace", "region", "id"]) {
    const value = readNonEmptyString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultDigitalOceanDeployCommand(): string {
  return "doctl serverless deploy . --remote-build --output json";
}

function defaultDigitalOceanDestroyCommand(): string {
  return "doctl serverless undeploy . --output json";
}

function resolveRegion(project: ProjectConfig): string {
  const extension = project.extensions?.["digitalocean-functions"];
  return typeof extension?.region === "string" ? extension.region : process.env.DIGITALOCEAN_REGION || "nyc1";
}

function resolveNamespace(project: ProjectConfig): string {
  const extension = project.extensions?.["digitalocean-functions"];
  if (typeof extension?.namespace === "string") {
    return extension.namespace;
  }
  return process.env.DIGITALOCEAN_NAMESPACE || "default";
}

function defaultEndpoint(service: string, region: string, namespace: string): string {
  return `https://faas-${region}.doserverless.co/api/v1/web/${namespace}/default/${service}`;
}

async function deployDigitalOceanFunctions(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const region = resolveRegion(project);
  const namespace = resolveNamespace(project);
  const deploymentId = createDeploymentId("digitalocean-functions", project.service, stage);
  const deployState = await runStandardCliRealDeployIfEnabled({
    projectDir: options.projectDir,
    project,
    plan,
    stage,
    realDeployEnv: "RUNFABRIC_DIGITALOCEAN_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_DIGITALOCEAN_DEPLOY_CMD",
    defaultDeployCommand: defaultDigitalOceanDeployCommand(),
    defaultEndpoint: defaultEndpoint(project.service, region, namespace),
    parseEndpoint: endpointFromDoResponse,
    missingEndpointError: "digitalocean-functions deploy response does not include endpoint",
    buildResource: (rawResponse) => doResourceMetadata(rawResponse),
    extraResource: {
      namespace,
      region
    }
  });

  await writeDeploymentReceipt(options.projectDir, "digitalocean-functions", {
    provider: "digitalocean-functions",
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
    "digitalocean-functions",
    `deploy deploymentId=${deploymentId} mode=${deployState.mode} endpoint=${deployState.endpoint}`
  );

  return { provider: "digitalocean-functions", endpoint: deployState.endpoint };
}

async function destroyDigitalOceanFunctions(options: ProviderOptions, project: ProjectConfig): Promise<void> {
  if (isRealDeployModeEnabled("RUNFABRIC_DIGITALOCEAN_REAL_DEPLOY")) {
    const stage = project.stage || "default";
    const destroyCommand =
      process.env.RUNFABRIC_DIGITALOCEAN_DESTROY_CMD || defaultDigitalOceanDestroyCommand();
    const result = await runShellCommand(destroyCommand, {
      cwd: options.projectDir,
      env: {
        RUNFABRIC_SERVICE: project.service,
        RUNFABRIC_STAGE: stage
      }
    });
    if (result.code !== 0) {
      throw new Error(result.stderr || result.stdout || "digitalocean-functions destroy command failed");
    }
  }

  await appendProviderLog(options.projectDir, "digitalocean-functions", "destroy local artifacts");
  await destroyProviderArtifacts(options.projectDir, "digitalocean-functions");
}

function validateDigitalOceanProvider(): ValidationResult {
  const errors = missingRequiredCredentialErrors(digitalOceanCredentialSchema);
  return { ok: errors.length === 0, warnings: [], errors };
}

const digitalOceanPlanOperations = createStandardProviderPlanOperations(
  "digitalocean-functions",
  "prepare digitalocean functions metadata"
);

export function createDigitalOceanFunctionsProvider(options: ProviderOptions): ProviderAdapter {
  const observabilityOperations = createProviderNativeObservabilityOperations({
    projectDir: options.projectDir,
    provider: "digitalocean-functions",
    realDeployEnv: "RUNFABRIC_DIGITALOCEAN_REAL_DEPLOY",
    tracesCommandEnv: "RUNFABRIC_DIGITALOCEAN_TRACES_CMD",
    metricsCommandEnv: "RUNFABRIC_DIGITALOCEAN_METRICS_CMD"
  });

  return {
    name: "digitalocean-functions",
    getCapabilities: () => digitalOceanFunctionsCapabilities,
    getCredentialSchema: () => digitalOceanCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateDigitalOceanProvider(),
    planBuild: digitalOceanPlanOperations.planBuild,
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: digitalOceanPlanOperations.planDeploy,
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployDigitalOceanFunctions(options, project, plan),
    invoke: async (input) =>
      invokeProviderViaDeployedEndpoint(options.projectDir, "digitalocean-functions", input),
    logs: async (input) =>
      buildProviderLogsFromLocalArtifacts(options.projectDir, "digitalocean-functions", input),
    traces: observabilityOperations.traces,
    metrics: observabilityOperations.metrics,
    destroy: async (project: ProjectConfig) => destroyDigitalOceanFunctions(options, project)
  };
}
