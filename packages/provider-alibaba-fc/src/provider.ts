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
import { alibabaFcCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const alibabaCredentialSchema: ProviderCredentialSchema = {
  provider: "alibaba-fc",
  fields: [
    { env: "ALICLOUD_ACCESS_KEY_ID", description: "Alibaba Cloud access key ID" },
    { env: "ALICLOUD_ACCESS_KEY_SECRET", description: "Alibaba Cloud access key secret" },
    { env: "ALICLOUD_REGION", description: "Alibaba Cloud region for Function Compute" }
  ]
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function toHttpEndpoint(value: string): string {
  if (value.startsWith("http://") || value.startsWith("https://")) {
    return value;
  }
  return `https://${value}`;
}

function endpointFromAlibabaResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const direct = readString(response.endpoint) || readString(response.url) || readString(response.internetAddress);
  if (direct) {
    return toHttpEndpoint(direct);
  }

  if (isRecord(response.result)) {
    const nested = readString(response.result.url) || readString(response.result.endpoint);
    return nested ? toHttpEndpoint(nested) : undefined;
  }

  return undefined;
}

function alibabaResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["serviceName", "functionName", "region", "requestId"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultAlibabaDeployCommand(): string {
  return "s deploy --output json";
}

function defaultAlibabaDestroyCommand(): string {
  return "s remove --yes";
}

function resolveRegion(project: ProjectConfig): string {
  const extension = project.extensions?.["alibaba-fc"];
  if (typeof extension?.region === "string") {
    return extension.region;
  }
  return process.env.ALICLOUD_REGION || "cn-hangzhou";
}

async function runRealDeployIfEnabled(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan,
  stage: string,
  region: string
): Promise<{
  endpoint: string;
  mode: "simulated" | "cli";
  rawResponse?: unknown;
  resource?: Record<string, unknown>;
}> {
  const defaultEndpoint = `https://${project.service}.${region}.fcapp.run`;
  if (!isRealDeployModeEnabled("RUNFABRIC_ALIBABA_REAL_DEPLOY")) {
    return { endpoint: defaultEndpoint, mode: "simulated" };
  }

  const deployCommand = process.env.RUNFABRIC_ALIBABA_DEPLOY_CMD || defaultAlibabaDeployCommand();
  const hasCommandOverride = Boolean(process.env.RUNFABRIC_ALIBABA_DEPLOY_CMD);
  const rawResponse = await runJsonCommand(deployCommand, {
    cwd: options.projectDir,
    env: {
      RUNFABRIC_SERVICE: project.service,
      RUNFABRIC_STAGE: stage,
      RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
      RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath
    }
  });

  const parsedEndpoint = endpointFromAlibabaResponse(rawResponse);
  if (!parsedEndpoint && hasCommandOverride) {
    throw new Error("alibaba-fc deploy response does not include endpoint");
  }

  return {
    endpoint: parsedEndpoint || defaultEndpoint,
    mode: "cli",
    rawResponse,
    resource: {
      ...(alibabaResourceMetadata(rawResponse) || {}),
      region,
      deployCommandSource: hasCommandOverride ? "override" : "builtin"
    }
  };
}

async function deployAlibabaFc(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const region = resolveRegion(project);
  const deploymentId = createDeploymentId("alibaba-fc", project.service, stage);
  const deployState = await runRealDeployIfEnabled(options, project, plan, stage, region);

  await writeDeploymentReceipt(options.projectDir, "alibaba-fc", {
    provider: "alibaba-fc",
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
    "alibaba-fc",
    `deploy deploymentId=${deploymentId} mode=${deployState.mode} endpoint=${deployState.endpoint}`
  );

  return { provider: "alibaba-fc", endpoint: deployState.endpoint };
}

async function destroyAlibabaFc(options: ProviderOptions, project: ProjectConfig): Promise<void> {
  if (isRealDeployModeEnabled("RUNFABRIC_ALIBABA_REAL_DEPLOY")) {
    const stage = project.stage || "default";
    const destroyCommand = process.env.RUNFABRIC_ALIBABA_DESTROY_CMD || defaultAlibabaDestroyCommand();
    const result = await runShellCommand(destroyCommand, {
      cwd: options.projectDir,
      env: {
        RUNFABRIC_SERVICE: project.service,
        RUNFABRIC_STAGE: stage
      }
    });
    if (result.code !== 0) {
      throw new Error(result.stderr || result.stdout || "alibaba-fc destroy command failed");
    }
  }

  await appendProviderLog(options.projectDir, "alibaba-fc", "destroy local artifacts");
  await destroyProviderArtifacts(options.projectDir, "alibaba-fc");
}

function validateAlibabaProvider(): ValidationResult {
  const errors = missingRequiredCredentialErrors(alibabaCredentialSchema);
  return { ok: errors.length === 0, warnings: [], errors };
}

function createBuildPlan(): BuildPlan {
  return {
    provider: "alibaba-fc",
    steps: ["prepare alibaba fc metadata"]
  };
}

function createDeployPlan(artifact: BuildArtifact): DeployPlan {
  return {
    provider: "alibaba-fc",
    artifactPath: artifact.entry,
    artifactManifestPath: artifact.outputPath,
    steps: [`deploy artifact from ${artifact.outputPath}`]
  };
}

export function createAlibabaFcProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "alibaba-fc",
    getCapabilities: () => alibabaFcCapabilities,
    getCredentialSchema: () => alibabaCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateAlibabaProvider(),
    planBuild: async (): Promise<BuildPlan> => createBuildPlan(),
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: async (_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> =>
      createDeployPlan(artifact),
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployAlibabaFc(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "alibaba-fc", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "alibaba-fc", input),
    destroy: async (project: ProjectConfig) => destroyAlibabaFc(options, project)
  };
}
