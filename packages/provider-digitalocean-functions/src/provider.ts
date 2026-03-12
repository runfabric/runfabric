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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function endpointFromDoResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const endpoint = readString(response.endpoint) || readString(response.url) || readString(response.webUrl);
  if (endpoint) {
    return endpoint;
  }

  if (isRecord(response.result)) {
    return readString(response.result.url) || readString(response.result.endpoint);
  }

  return undefined;
}

function doResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["name", "namespace", "region", "id"]) {
    const value = readString(response[key]);
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

async function runRealDeployIfEnabled(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan,
  stage: string,
  region: string,
  namespace: string
): Promise<{
  endpoint: string;
  mode: "simulated" | "cli";
  rawResponse?: unknown;
  resource?: Record<string, unknown>;
}> {
  const initialEndpoint = defaultEndpoint(project.service, region, namespace);
  if (!isRealDeployModeEnabled("RUNFABRIC_DIGITALOCEAN_REAL_DEPLOY")) {
    return { endpoint: initialEndpoint, mode: "simulated" };
  }

  const deployCommand = process.env.RUNFABRIC_DIGITALOCEAN_DEPLOY_CMD || defaultDigitalOceanDeployCommand();
  const hasCommandOverride = Boolean(process.env.RUNFABRIC_DIGITALOCEAN_DEPLOY_CMD);
  const rawResponse = await runJsonCommand(deployCommand, {
    cwd: options.projectDir,
    env: {
      RUNFABRIC_SERVICE: project.service,
      RUNFABRIC_STAGE: stage,
      RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
      RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath
    }
  });

  const parsedEndpoint = endpointFromDoResponse(rawResponse);
  if (!parsedEndpoint && hasCommandOverride) {
    throw new Error("digitalocean-functions deploy response does not include endpoint");
  }

  return {
    endpoint: parsedEndpoint || initialEndpoint,
    mode: "cli",
    rawResponse,
    resource: {
      ...(doResourceMetadata(rawResponse) || {}),
      namespace,
      region,
      deployCommandSource: hasCommandOverride ? "override" : "builtin"
    }
  };
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
  const deployState = await runRealDeployIfEnabled(options, project, plan, stage, region, namespace);

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

function createBuildPlan(): BuildPlan {
  return {
    provider: "digitalocean-functions",
    steps: ["prepare digitalocean functions metadata"]
  };
}

function createDeployPlan(artifact: BuildArtifact): DeployPlan {
  return {
    provider: "digitalocean-functions",
    artifactPath: artifact.entry,
    artifactManifestPath: artifact.outputPath,
    steps: [`deploy artifact from ${artifact.outputPath}`]
  };
}

export function createDigitalOceanFunctionsProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "digitalocean-functions",
    getCapabilities: () => digitalOceanFunctionsCapabilities,
    getCredentialSchema: () => digitalOceanCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateDigitalOceanProvider(),
    planBuild: async (): Promise<BuildPlan> => createBuildPlan(),
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: async (_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> =>
      createDeployPlan(artifact),
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployDigitalOceanFunctions(options, project, plan),
    invoke: async (input) =>
      invokeProviderViaDeployedEndpoint(options.projectDir, "digitalocean-functions", input),
    logs: async (input) =>
      buildProviderLogsFromLocalArtifacts(options.projectDir, "digitalocean-functions", input),
    destroy: async (project: ProjectConfig) => destroyDigitalOceanFunctions(options, project)
  };
}
