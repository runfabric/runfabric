import type { Runtime } from "@aws-sdk/client-lambda";
import type {
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
  buildProviderMetricsFromLocalArtifacts,
  buildProviderLogsFromLocalArtifacts,
  buildProviderTracesFromLocalArtifacts,
  createStandardProviderPlanOperations,
  createDeploymentId,
  destroyProviderArtifacts,
  isRecordLike,
  invokeProviderViaDeployedEndpoint,
  isRealDeployModeEnabled,
  missingRequiredCredentialErrors,
  readFiniteNumber,
  readNonEmptyString,
  readDeploymentReceipt,
  runJsonCommand,
  runShellCommand,
  TriggerEnum,
  writeDeploymentReceipt
} from "@runfabric/core";
import { awsLambdaCapabilities } from "./capabilities";
import { deployWithAwsSdk, destroyWithAwsSdk } from "./deploy-internal";
import {
  awsResourceMetadata,
  collectResourceAddresses,
  collectSecretReferences,
  collectWorkflowAddresses,
  createAwsDeployMetadata,
  endpointFromAwsResponse
} from "./provider-metadata";

interface AwsProviderOptions {
  projectDir: string;
}

interface AwsLambdaExtensionConfig {
  stage?: string;
  region?: string;
  functionName?: string;
  roleArn?: string;
  runtime?: string;
  timeout?: number;
  memory?: number;
  [key: string]: unknown;
}

interface AwsDeployExecution {
  endpoint: string;
  mode: "simulated" | "cli" | "api";
  rawResponse?: unknown;
  resource?: Record<string, unknown>;
}

interface AwsDeploySummary {
  stage: string;
  region: string;
  deploymentId: string;
  execution: AwsDeployExecution;
  metadata: ReturnType<typeof createAwsDeployMetadata>;
}

const awsLambdaCredentialSchema: ProviderCredentialSchema = {
  provider: "aws-lambda",
  fields: [
    { env: "AWS_ACCESS_KEY_ID", description: "AWS IAM access key ID" },
    { env: "AWS_SECRET_ACCESS_KEY", description: "AWS IAM secret access key" },
    { env: "AWS_REGION", description: "AWS region for Lambda deployment" }
  ]
};

function awsExtension(project: ProjectConfig): AwsLambdaExtensionConfig {
  const extension = project.extensions?.["aws-lambda"];
  return isRecordLike(extension) ? (extension as AwsLambdaExtensionConfig) : {};
}

function resolveStage(project: ProjectConfig): string {
  const extension = awsExtension(project);
  return readNonEmptyString(extension.stage) || project.stage || "default";
}

function resolveRegion(project: ProjectConfig): string {
  const extension = awsExtension(project);
  return readNonEmptyString(extension.region) || process.env.AWS_REGION || "us-east-1";
}

function sanitizeLambdaFunctionName(value: string): string {
  const sanitized = value.replace(/[^a-zA-Z0-9-_]/g, "-").replace(/--+/g, "-").replace(/^-+/, "").replace(/-+$/, "");
  return sanitized ? sanitized.slice(0, 64) : "runfabric-fn";
}

function resolveFunctionName(project: ProjectConfig, stage: string): string {
  const extension = awsExtension(project);
  const configured = readNonEmptyString(extension.functionName);
  return configured ? sanitizeLambdaFunctionName(configured) : sanitizeLambdaFunctionName(`${project.service}-${stage}`);
}

function resolveRoleArn(project: ProjectConfig): string | undefined {
  const extension = awsExtension(project);
  return (
    readNonEmptyString(extension.roleArn) ||
    readNonEmptyString(process.env.RUNFABRIC_AWS_LAMBDA_ROLE_ARN)
  );
}

function resolveLambdaRuntime(project: ProjectConfig): Runtime {
  const extension = awsExtension(project);
  const runtime =
    readNonEmptyString(extension.runtime) ||
    readNonEmptyString(process.env.RUNFABRIC_AWS_LAMBDA_RUNTIME) ||
    "nodejs20.x";
  return runtime as Runtime;
}

function resolveLambdaTimeout(project: ProjectConfig): number | undefined {
  const extension = awsExtension(project);
  const timeout = readFiniteNumber(extension.timeout) || readFiniteNumber(project.resources?.timeout);
  return typeof timeout === "number" ? Math.max(1, Math.floor(timeout)) : undefined;
}

function resolveLambdaMemory(project: ProjectConfig): number | undefined {
  const extension = awsExtension(project);
  const memory = readFiniteNumber(extension.memory) || readFiniteNumber(project.resources?.memory);
  return typeof memory === "number" ? Math.max(128, Math.floor(memory)) : undefined;
}

function hasSupportedTrigger(project: ProjectConfig): boolean {
  return project.triggers.some((trigger) =>
    [
      TriggerEnum.Http,
      TriggerEnum.Cron,
      TriggerEnum.Queue,
      TriggerEnum.Storage,
      TriggerEnum.EventBridge
    ].includes(trigger.type)
  );
}

function validateAwsProject(project: ProjectConfig): ValidationResult {
  const warnings: string[] = [];
  const errors: string[] = [];

  if (!project.providers.includes("aws-lambda")) {
    warnings.push("project does not target aws-lambda in providers");
  }
  if (!hasSupportedTrigger(project)) {
    warnings.push("aws-lambda provider has no supported triggers configured");
  }

  errors.push(...missingRequiredCredentialErrors(awsLambdaCredentialSchema));
  return { ok: errors.length === 0, warnings, errors };
}

function buildAwsArtifacts(plan: BuildPlan): BuildResult {
  return {
    artifacts: plan.steps.map((step, index) => ({
      provider: "aws-lambda",
      entry: step,
      outputPath: `aws-lambda-step-${index + 1}`
    }))
  };
}

const awsPlanOperations = createStandardProviderPlanOperations("aws-lambda", [
  "validate config",
  "prepare aws-lambda artifact manifest"
]);

function buildCliDeployEnv(input: {
  project: ProjectConfig;
  plan: DeployPlan;
  stage: string;
  metadata: ReturnType<typeof createAwsDeployMetadata>;
}): Record<string, string> {
  return {
    RUNFABRIC_SERVICE: input.project.service,
    RUNFABRIC_STAGE: input.stage,
    RUNFABRIC_ARTIFACT_PATH: input.plan.artifactPath || "",
    RUNFABRIC_ARTIFACT_MANIFEST_PATH: input.plan.artifactManifestPath || "",
    RUNFABRIC_AWS_QUEUE_EVENT_SOURCES_JSON: JSON.stringify(input.metadata.queueEventSources),
    RUNFABRIC_AWS_STORAGE_EVENTS_JSON: JSON.stringify(input.metadata.storageEvents),
    RUNFABRIC_AWS_EVENTBRIDGE_RULES_JSON: JSON.stringify(input.metadata.eventBridgeRules),
    RUNFABRIC_AWS_IAM_ROLE_STATEMENTS_JSON: JSON.stringify(input.metadata.iamRoleStatements),
    RUNFABRIC_FUNCTION_ENV_JSON: JSON.stringify(input.metadata.functionEnv),
    RUNFABRIC_AWS_RESOURCE_ADDRESSES_JSON: JSON.stringify(input.metadata.resourceAddresses),
    RUNFABRIC_AWS_WORKFLOW_ADDRESSES_JSON: JSON.stringify(input.metadata.workflowAddresses),
    RUNFABRIC_AWS_SECRET_REFERENCES_JSON: JSON.stringify(input.metadata.secretReferences)
  };
}

async function runAwsCliDeploy(input: {
  options: AwsProviderOptions;
  project: ProjectConfig;
  plan: DeployPlan;
  stage: string;
  metadata: ReturnType<typeof createAwsDeployMetadata>;
}): Promise<AwsDeployExecution> {
  const command = process.env.RUNFABRIC_AWS_DEPLOY_CMD;
  if (!command) {
    throw new Error("aws-lambda CLI deploy requested without RUNFABRIC_AWS_DEPLOY_CMD");
  }

  const rawResponse = await runJsonCommand(command, {
    cwd: input.options.projectDir,
    env: buildCliDeployEnv(input)
  });
  const endpoint = endpointFromAwsResponse(rawResponse);
  if (!endpoint) {
    throw new Error("aws-lambda deploy response does not include an endpoint/function URL");
  }

  return {
    endpoint,
    mode: "cli",
    rawResponse,
    resource: awsResourceMetadata(rawResponse)
  };
}

async function runAwsApiDeploy(input: {
  project: ProjectConfig;
  plan: DeployPlan;
  stage: string;
  region: string;
  metadata: ReturnType<typeof createAwsDeployMetadata>;
  projectDir: string;
}): Promise<AwsDeployExecution> {
  const roleArn = resolveRoleArn(input.project);
  if (!roleArn) {
    throw new Error(
      "aws-lambda internal deploy requires a role ARN. Set extensions.aws-lambda.roleArn or RUNFABRIC_AWS_LAMBDA_ROLE_ARN."
    );
  }

  const internalDeploy = await deployWithAwsSdk({
    projectDir: input.projectDir,
    plan: input.plan,
    stage: input.stage,
    region: input.region,
    roleArn,
    functionName: resolveFunctionName(input.project, input.stage),
    runtime: resolveLambdaRuntime(input.project),
    timeout: resolveLambdaTimeout(input.project),
    memory: resolveLambdaMemory(input.project),
    functionEnv: input.metadata.functionEnv
  });

  return {
    endpoint: internalDeploy.endpoint,
    mode: "api",
    rawResponse: internalDeploy.rawResponse,
    resource: internalDeploy.resource
  };
}

function simulatedEndpoint(project: ProjectConfig, stage: string, region: string): string {
  return `https://${project.service}.execute-api.${region}.amazonaws.com/${stage}`;
}

function enrichResource(
  resource: Record<string, unknown> | undefined,
  metadata: ReturnType<typeof createAwsDeployMetadata>
): Record<string, unknown> {
  return {
    ...(resource || {}),
    queueEventSources: metadata.queueEventSources,
    storageEvents: metadata.storageEvents,
    eventBridgeRules: metadata.eventBridgeRules,
    iamRoleStatements: metadata.iamRoleStatements,
    functionEnvKeys: Object.keys(metadata.functionEnv),
    resourceAddresses: metadata.resourceAddresses,
    workflowAddresses: metadata.workflowAddresses,
    secretReferences: metadata.secretReferences
  };
}

async function executeAwsDeploy(
  options: AwsProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<AwsDeploySummary> {
  const stage = resolveStage(project);
  const region = resolveRegion(project);
  const metadata = createAwsDeployMetadata(project, region, stage);
  const deploymentId = createDeploymentId("aws-lambda", project.service, stage);

  const execution = !isRealDeployModeEnabled("RUNFABRIC_AWS_REAL_DEPLOY")
    ? { endpoint: simulatedEndpoint(project, stage, region), mode: "simulated" as const }
    : process.env.RUNFABRIC_AWS_DEPLOY_CMD
      ? await runAwsCliDeploy({ options, project, plan, stage, metadata })
      : await runAwsApiDeploy({ project, plan, stage, region, metadata, projectDir: options.projectDir });

  return {
    stage,
    region,
    deploymentId,
    execution,
    metadata
  };
}

async function persistAwsDeployment(
  options: AwsProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan,
  summary: AwsDeploySummary
): Promise<void> {
  await writeDeploymentReceipt(options.projectDir, "aws-lambda", {
    provider: "aws-lambda",
    service: project.service,
    stage: summary.stage,
    deploymentId: summary.deploymentId,
    endpoint: summary.execution.endpoint,
    mode: summary.execution.mode,
    executedSteps: plan.steps,
    artifactPath: plan.artifactPath,
    artifactManifestPath: plan.artifactManifestPath,
    resource: enrichResource(summary.execution.resource, summary.metadata),
    rawResponse: summary.execution.rawResponse,
    createdAt: new Date().toISOString()
  });

  await appendProviderLog(
    options.projectDir,
    "aws-lambda",
    `deploy deploymentId=${summary.deploymentId} mode=${summary.execution.mode} endpoint=${summary.execution.endpoint}`
  );
}

async function runAwsDestroy(options: AwsProviderOptions, project: ProjectConfig): Promise<void> {
  const stage = resolveStage(project);
  const region = resolveRegion(project);
  const realDeployModeEnabled = isRealDeployModeEnabled("RUNFABRIC_AWS_REAL_DEPLOY");

  if (!realDeployModeEnabled) {
    const receipt = await readDeploymentReceipt(options.projectDir, "aws-lambda");
    if (receipt && receipt.mode !== "simulated") {
      throw new Error(
        `aws-lambda destroy skipped cloud deletion because real deploy mode is disabled. Last deployment mode=${receipt.mode}. Set RUNFABRIC_AWS_REAL_DEPLOY=1 and rerun runfabric remove.`
      );
    }
    return;
  }

  if (process.env.RUNFABRIC_AWS_DESTROY_CMD) {
    const result = await runShellCommand(process.env.RUNFABRIC_AWS_DESTROY_CMD, {
      cwd: options.projectDir,
      env: { RUNFABRIC_SERVICE: project.service, RUNFABRIC_STAGE: stage }
    });
    if (result.code !== 0) {
      throw new Error(result.stderr || result.stdout || "aws-lambda destroy command failed");
    }
    return;
  }

  await destroyWithAwsSdk(resolveFunctionName(project, stage), region);
}

function createAwsLambdaProviderAdapter(options: AwsProviderOptions): ProviderAdapter {
  return {
    name: "aws-lambda",
    getCapabilities: () => awsLambdaCapabilities,
    getCredentialSchema: () => awsLambdaCredentialSchema,
    validate: async (project) => validateAwsProject(project),
    provisionResources: async (project) => ({
      provider: "aws-lambda",
      resourceAddresses: collectResourceAddresses(project, resolveRegion(project))
    }),
    deployWorkflows: async (project) => {
      const stage = resolveStage(project);
      const region = resolveRegion(project);
      return { provider: "aws-lambda", workflowAddresses: collectWorkflowAddresses(project, region, stage) };
    },
    materializeSecrets: async (project) => ({
      provider: "aws-lambda",
      secretReferences: collectSecretReferences(project, resolveRegion(project))
    }),
    planBuild: awsPlanOperations.planBuild,
    build: async (_project, plan) => buildAwsArtifacts(plan),
    planDeploy: awsPlanOperations.planDeploy,
    deploy: async (project, plan) => {
      const summary = await executeAwsDeploy(options, project, plan);
      await persistAwsDeployment(options, project, plan, summary);
      return {
        provider: "aws-lambda",
        endpoint: summary.execution.endpoint,
        resourceAddresses: summary.metadata.resourceAddresses,
        workflowAddresses: summary.metadata.workflowAddresses,
        secretReferences: summary.metadata.secretReferences
      };
    },
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "aws-lambda", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "aws-lambda", input),
    traces: async (input) => buildProviderTracesFromLocalArtifacts(options.projectDir, "aws-lambda", input),
    metrics: async (input) => buildProviderMetricsFromLocalArtifacts(options.projectDir, "aws-lambda", input),
    destroy: async (project) => {
      await runAwsDestroy(options, project);
      await appendProviderLog(options.projectDir, "aws-lambda", "destroy local artifacts");
      await destroyProviderArtifacts(options.projectDir, "aws-lambda");
    }
  };
}

export function createAwsLambdaProvider(options: AwsProviderOptions): ProviderAdapter {
  return createAwsLambdaProviderAdapter(options);
}
