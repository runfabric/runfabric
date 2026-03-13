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

function toHttpEndpoint(value: string): string {
  if (value.startsWith("http://") || value.startsWith("https://")) {
    return value;
  }
  return `https://${value}`;
}

function endpointFromAlibabaResponse(response: unknown): string | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const direct =
    readNonEmptyString(response.endpoint) ||
    readNonEmptyString(response.url) ||
    readNonEmptyString(response.internetAddress);
  if (direct) {
    return toHttpEndpoint(direct);
  }

  if (isRecordLike(response.result)) {
    const nested = readNonEmptyString(response.result.url) || readNonEmptyString(response.result.endpoint);
    return nested ? toHttpEndpoint(nested) : undefined;
  }

  return undefined;
}

function alibabaResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["serviceName", "functionName", "region", "requestId"]) {
    const value = readNonEmptyString(response[key]);
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

async function deployAlibabaFc(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const region = resolveRegion(project);
  const deploymentId = createDeploymentId("alibaba-fc", project.service, stage);
  const deployState = await runStandardCliRealDeployIfEnabled({
    projectDir: options.projectDir,
    project,
    plan,
    stage,
    realDeployEnv: "RUNFABRIC_ALIBABA_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_ALIBABA_DEPLOY_CMD",
    defaultDeployCommand: defaultAlibabaDeployCommand(),
    defaultEndpoint: `https://${project.service}.${region}.fcapp.run`,
    parseEndpoint: endpointFromAlibabaResponse,
    missingEndpointError: "alibaba-fc deploy response does not include endpoint",
    buildResource: (rawResponse) => alibabaResourceMetadata(rawResponse),
    extraResource: { region }
  });

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

const alibabaPlanOperations = createStandardProviderPlanOperations(
  "alibaba-fc",
  "prepare alibaba fc metadata"
);

export function createAlibabaFcProvider(options: ProviderOptions): ProviderAdapter {
  const observabilityOperations = createProviderNativeObservabilityOperations({
    projectDir: options.projectDir,
    provider: "alibaba-fc",
    realDeployEnv: "RUNFABRIC_ALIBABA_REAL_DEPLOY",
    tracesCommandEnv: "RUNFABRIC_ALIBABA_TRACES_CMD",
    metricsCommandEnv: "RUNFABRIC_ALIBABA_METRICS_CMD"
  });

  return {
    name: "alibaba-fc",
    getCapabilities: () => alibabaFcCapabilities,
    getCredentialSchema: () => alibabaCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateAlibabaProvider(),
    planBuild: alibabaPlanOperations.planBuild,
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: alibabaPlanOperations.planDeploy,
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployAlibabaFc(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "alibaba-fc", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "alibaba-fc", input),
    traces: observabilityOperations.traces,
    metrics: observabilityOperations.metrics,
    destroy: async (project: ProjectConfig) => destroyAlibabaFc(options, project)
  };
}
