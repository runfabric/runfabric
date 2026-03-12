import { readdir, readFile } from "node:fs/promises";
import { basename, dirname, extname, isAbsolute, relative, resolve } from "node:path";
import {
  AddPermissionCommand,
  CreateFunctionCommand,
  CreateFunctionUrlConfigCommand,
  DeleteFunctionCommand,
  DeleteFunctionUrlConfigCommand,
  GetFunctionCommand,
  GetFunctionUrlConfigCommand,
  LambdaClient,
  RemovePermissionCommand,
  UpdateFunctionCodeCommand,
  UpdateFunctionConfigurationCommand,
  waitUntilFunctionActiveV2,
  waitUntilFunctionUpdatedV2,
  type Runtime
} from "@aws-sdk/client-lambda";
import JSZip from "jszip";
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
  AwsIamEffectEnum,
  AwsQueueFunctionResponseTypeEnum,
  buildProviderLogsFromLocalArtifacts,
  createDeploymentId,
  destroyProviderArtifacts,
  invokeProviderViaDeployedEndpoint,
  isRealDeployModeEnabled,
  missingRequiredCredentialErrors,
  readDeploymentReceipt,
  runJsonCommand,
  runShellCommand,
  TriggerEnum,
  writeDeploymentReceipt
} from "@runfabric/core";
import { awsLambdaCapabilities } from "./capabilities";

interface AwsProviderOptions {
  projectDir: string;
}

interface AwsIamRoleStatement {
  sid?: string;
  effect: AwsIamEffectEnum;
  actions: string[];
  resources: string[];
  condition?: Record<string, unknown>;
}

const awsLambdaCredentialSchema: ProviderCredentialSchema = {
  provider: "aws-lambda",
  fields: [
    { env: "AWS_ACCESS_KEY_ID", description: "AWS IAM access key ID" },
    { env: "AWS_SECRET_ACCESS_KEY", description: "AWS IAM secret access key" },
    { env: "AWS_REGION", description: "AWS region for Lambda deployment" }
  ]
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function readNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function readBoolean(value: unknown): boolean | undefined {
  return typeof value === "boolean" ? value : undefined;
}

function readQueueFunctionResponseType(
  value: unknown
): AwsQueueFunctionResponseTypeEnum | undefined {
  if (value === AwsQueueFunctionResponseTypeEnum.ReportBatchItemFailures) {
    return AwsQueueFunctionResponseTypeEnum.ReportBatchItemFailures;
  }
  return undefined;
}

function endpointFromAwsResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const directCandidates = [
    response.endpoint,
    response.url,
    response.functionUrl,
    response.FunctionUrl
  ];

  for (const candidate of directCandidates) {
    const endpoint = readString(candidate);
    if (endpoint) {
      return endpoint;
    }
  }

  if (isRecord(response.FunctionUrlConfig)) {
    return readString(response.FunctionUrlConfig.FunctionUrl);
  }

  return undefined;
}

function awsResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["FunctionArn", "RevisionId", "Version", "Runtime"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function collectQueueEventSources(project: ProjectConfig): Array<Record<string, unknown>> {
  return project.triggers
    .filter((trigger) => trigger.type === TriggerEnum.Queue)
    .map((trigger) => ({
      queue: readString(trigger.queue),
      batchSize: readNumber(trigger.batchSize),
      maximumBatchingWindowSeconds: readNumber(trigger.maximumBatchingWindowSeconds),
      maximumConcurrency: readNumber(trigger.maximumConcurrency),
      enabled: readBoolean(trigger.enabled) ?? true,
      functionResponseType: readQueueFunctionResponseType(trigger.functionResponseType)
    }))
    .filter((source) => typeof source.queue === "string");
}

function collectStorageEvents(project: ProjectConfig): Array<Record<string, unknown>> {
  return project.triggers
    .filter((trigger) => trigger.type === TriggerEnum.Storage)
    .map((trigger) => {
      const events =
        Array.isArray(trigger.events) && trigger.events.every((entry) => typeof entry === "string")
          ? trigger.events
          : [];
      return {
        bucket: readString(trigger.bucket),
        events,
        prefix: readString(trigger.prefix),
        suffix: readString(trigger.suffix),
        existingBucket: readBoolean(trigger.existingBucket) ?? true
      };
    })
    .filter((source) => typeof source.bucket === "string" && Array.isArray(source.events) && source.events.length > 0);
}

function collectEventBridgeRules(project: ProjectConfig): Array<Record<string, unknown>> {
  return project.triggers
    .filter((trigger) => trigger.type === TriggerEnum.EventBridge)
    .map((trigger) => ({
      bus: readString(trigger.bus) || "default",
      pattern: isRecord(trigger.pattern) ? trigger.pattern : undefined
    }))
    .filter((rule) => isRecord(rule.pattern));
}

function collectIamRoleStatements(project: ProjectConfig): AwsIamRoleStatement[] {
  const extension = project.extensions?.["aws-lambda"];
  if (!extension || typeof extension !== "object") {
    return [];
  }

  const iam = (extension as { iam?: unknown }).iam;
  if (!iam || !isRecord(iam)) {
    return [];
  }

  const role = iam.role;
  if (!role || !isRecord(role) || !Array.isArray(role.statements)) {
    return [];
  }

  return role.statements.filter((statement): statement is AwsIamRoleStatement => {
    if (!isRecord(statement)) {
      return false;
    }
    const effect = readString(statement.effect);
    if (!effect || ![AwsIamEffectEnum.Allow, AwsIamEffectEnum.Deny].includes(effect as AwsIamEffectEnum)) {
      return false;
    }
    const actions = statement.actions;
    const resources = statement.resources;
    if (!Array.isArray(actions) || !actions.every((entry) => typeof entry === "string")) {
      return false;
    }
    if (!Array.isArray(resources) || !resources.every((entry) => typeof entry === "string")) {
      return false;
    }
    return true;
  });
}

function collectFunctionEnv(project: ProjectConfig): Record<string, string> {
  return { ...(project.env || {}) };
}

function sanitizeIdentifier(value: string): string {
  return value.replace(/[^a-zA-Z0-9-_./]/g, "-");
}

function resourceName(entry: unknown): string | undefined {
  if (!isRecord(entry)) {
    return undefined;
  }
  return readString(entry.name);
}

function collectResourceAddresses(
  project: ProjectConfig,
  region: string
): Record<string, string> {
  const accountId = "000000000000";
  const out: Record<string, string> = {};
  const resources = project.resources;
  if (!resources) {
    return out;
  }

  for (const queue of resources.queues || []) {
    const name = resourceName(queue);
    if (!name) {
      continue;
    }
    out[`queue.${name}`] = `arn:aws:sqs:${region}:${accountId}:${sanitizeIdentifier(name)}`;
  }
  for (const bucket of resources.buckets || []) {
    const name = resourceName(bucket);
    if (!name) {
      continue;
    }
    out[`bucket.${name}`] = `arn:aws:s3:::${sanitizeIdentifier(name)}`;
  }
  for (const topic of resources.topics || []) {
    const name = resourceName(topic);
    if (!name) {
      continue;
    }
    out[`topic.${name}`] = `arn:aws:sns:${region}:${accountId}:${sanitizeIdentifier(name)}`;
  }
  for (const database of resources.databases || []) {
    const name = resourceName(database);
    if (!name) {
      continue;
    }
    out[`database.${name}`] = `arn:aws:rds:${region}:${accountId}:db:${sanitizeIdentifier(name)}`;
  }

  return out;
}

function collectWorkflowAddresses(
  project: ProjectConfig,
  region: string,
  stage: string
): Record<string, string> {
  const accountId = "000000000000";
  const out: Record<string, string> = {};
  for (const workflow of project.workflows || []) {
    if (!workflow.name || workflow.name.trim().length === 0) {
      continue;
    }
    const name = sanitizeIdentifier(workflow.name.trim());
    out[`workflow.${workflow.name}`] =
      `arn:aws:states:${region}:${accountId}:stateMachine:${sanitizeIdentifier(project.service)}-${sanitizeIdentifier(stage)}-${name}`;
  }
  return out;
}

function collectSecretReferences(project: ProjectConfig, region: string): Record<string, string> {
  const accountId = "000000000000";
  const out: Record<string, string> = {};
  for (const [key, value] of Object.entries(project.secrets || {})) {
    if (typeof value !== "string" || !value.startsWith("secret://")) {
      continue;
    }
    const ref = value.slice("secret://".length).trim();
    if (!ref) {
      continue;
    }
    out[key] = `arn:aws:secretsmanager:${region}:${accountId}:secret:${sanitizeIdentifier(ref)}`;
  }
  return out;
}

function createAwsDeployMetadata(project: ProjectConfig, region: string, stage: string): {
  queueEventSources: Array<Record<string, unknown>>;
  storageEvents: Array<Record<string, unknown>>;
  eventBridgeRules: Array<Record<string, unknown>>;
  iamRoleStatements: AwsIamRoleStatement[];
  functionEnv: Record<string, string>;
  resourceAddresses: Record<string, string>;
  workflowAddresses: Record<string, string>;
  secretReferences: Record<string, string>;
} {
  return {
    queueEventSources: collectQueueEventSources(project),
    storageEvents: collectStorageEvents(project),
    eventBridgeRules: collectEventBridgeRules(project),
    iamRoleStatements: collectIamRoleStatements(project),
    functionEnv: collectFunctionEnv(project),
    resourceAddresses: collectResourceAddresses(project, region),
    workflowAddresses: collectWorkflowAddresses(project, region, stage),
    secretReferences: collectSecretReferences(project, region)
  };
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

function awsExtension(project: ProjectConfig): AwsLambdaExtensionConfig {
  const extension = project.extensions?.["aws-lambda"];
  return isRecord(extension) ? (extension as AwsLambdaExtensionConfig) : {};
}

function resolveStage(project: ProjectConfig): string {
  const extension = awsExtension(project);
  return readString(extension.stage) || project.stage || "default";
}

function resolveRegion(project: ProjectConfig): string {
  const extension = awsExtension(project);
  return readString(extension.region) || process.env.AWS_REGION || "us-east-1";
}

function sanitizeLambdaFunctionName(value: string): string {
  const sanitized = value.replace(/[^a-zA-Z0-9-_]/g, "-").replace(/--+/g, "-").replace(/^-+/, "").replace(/-+$/, "");
  if (!sanitized) {
    return "runfabric-fn";
  }
  return sanitized.slice(0, 64);
}

function resolveFunctionName(project: ProjectConfig, stage: string): string {
  const extension = awsExtension(project);
  const configured = readString(extension.functionName);
  if (configured) {
    return sanitizeLambdaFunctionName(configured);
  }
  return sanitizeLambdaFunctionName(`${project.service}-${stage}`);
}

function resolveRoleArn(project: ProjectConfig): string | undefined {
  const extension = awsExtension(project);
  return readString(extension.roleArn) || readString(process.env.RUNFABRIC_AWS_LAMBDA_ROLE_ARN);
}

function resolveLambdaRuntime(project: ProjectConfig): Runtime {
  const extension = awsExtension(project);
  const runtime = readString(extension.runtime) || readString(process.env.RUNFABRIC_AWS_LAMBDA_RUNTIME) || "nodejs20.x";
  return runtime as Runtime;
}

function resolveLambdaTimeout(project: ProjectConfig): number | undefined {
  const extension = awsExtension(project);
  const timeout = readNumber(extension.timeout) || readNumber(project.resources?.timeout);
  if (typeof timeout !== "number") {
    return undefined;
  }
  return Math.max(1, Math.floor(timeout));
}

function resolveLambdaMemory(project: ProjectConfig): number | undefined {
  const extension = awsExtension(project);
  const memory = readNumber(extension.memory) || readNumber(project.resources?.memory);
  if (typeof memory !== "number") {
    return undefined;
  }
  return Math.max(128, Math.floor(memory));
}

function resolveProjectPath(projectDir: string, filePath: string | undefined): string | undefined {
  if (!filePath || filePath.trim().length === 0) {
    return undefined;
  }
  return isAbsolute(filePath) ? filePath : resolve(projectDir, filePath);
}

async function collectFilesRecursively(rootDir: string): Promise<string[]> {
  const out: string[] = [];
  const entries = await readdir(rootDir, { withFileTypes: true });
  for (const entry of entries) {
    const absolutePath = resolve(rootDir, entry.name);
    if (entry.isDirectory()) {
      const nested = await collectFilesRecursively(absolutePath);
      out.push(...nested);
      continue;
    }
    if (entry.isFile()) {
      out.push(absolutePath);
    }
  }
  return out;
}

async function createZipFromArtifact(projectDir: string, plan: DeployPlan): Promise<Buffer> {
  const artifactManifestPath = resolveProjectPath(projectDir, plan.artifactManifestPath);
  if (!artifactManifestPath) {
    throw new Error("aws-lambda internal deploy requires artifactManifestPath");
  }

  const artifactRoot = dirname(artifactManifestPath);
  const files = await collectFilesRecursively(artifactRoot);
  if (files.length === 0) {
    throw new Error(`aws-lambda internal deploy could not find artifact files under ${artifactRoot}`);
  }

  const zip = new JSZip();
  for (const absolutePath of files) {
    const relativePath = relative(artifactRoot, absolutePath).replace(/\\/g, "/");
    if (!relativePath) {
      continue;
    }
    const content = await readFile(absolutePath);
    zip.file(relativePath, content);
  }

  return zip.generateAsync({
    type: "nodebuffer",
    compression: "DEFLATE",
    compressionOptions: { level: 9 }
  });
}

function resolveLambdaHandler(projectDir: string, plan: DeployPlan): string {
  const artifactPath = resolveProjectPath(projectDir, plan.artifactPath);
  const artifactManifestPath = resolveProjectPath(projectDir, plan.artifactManifestPath);
  if (!artifactPath) {
    throw new Error("aws-lambda internal deploy requires artifactPath");
  }
  if (!artifactManifestPath) {
    throw new Error("aws-lambda internal deploy requires artifactManifestPath");
  }

  const artifactRoot = dirname(artifactManifestPath);
  const artifactRelativePath = relative(artifactRoot, artifactPath).replace(/\\/g, "/");
  if (
    artifactRelativePath.length === 0 ||
    artifactRelativePath.startsWith("..") ||
    artifactRelativePath.startsWith("/")
  ) {
    throw new Error(
      `aws-lambda internal deploy expected artifactPath within artifact root: ${artifactPath} vs ${artifactRoot}`
    );
  }

  const artifactDir = dirname(artifactRelativePath).replace(/\\/g, "/");
  const extension = extname(artifactPath);
  const baseName = basename(artifactPath, extension);
  if (!baseName) {
    throw new Error(`cannot resolve lambda handler from artifactPath: ${artifactPath}`);
  }
  const modulePath = artifactDir === "." ? baseName : `${artifactDir}/${baseName}`;
  return `${modulePath}.handler`;
}

function isAwsErrorNamed(error: unknown, ...names: string[]): boolean {
  return isRecord(error) && typeof error.name === "string" && names.includes(error.name);
}

async function ensureFunctionUrl(
  client: LambdaClient,
  functionName: string,
  stage: string
): Promise<string> {
  const upsertPermission = async (
    input: {
      statementId: string;
      action: string;
      functionUrlAuthType?: "NONE" | "AWS_IAM";
      invokedViaFunctionUrl?: boolean;
    }
  ) => {
    const addPermission = async () => {
      await client.send(
        new AddPermissionCommand({
          FunctionName: functionName,
          StatementId: input.statementId,
          Action: input.action,
          Principal: "*",
          ...(input.functionUrlAuthType
            ? {
                FunctionUrlAuthType: input.functionUrlAuthType
              }
            : {}),
          ...(typeof input.invokedViaFunctionUrl === "boolean"
            ? {
                InvokedViaFunctionUrl: input.invokedViaFunctionUrl
              }
            : {})
        })
      );
    };

    try {
      await addPermission();
    } catch (error) {
      if (!isAwsErrorNamed(error, "ResourceConflictException")) {
        throw error;
      }
      try {
        await client.send(
          new RemovePermissionCommand({
            FunctionName: functionName,
            StatementId: input.statementId
          })
        );
      } catch (removeError) {
        if (!isAwsErrorNamed(removeError, "ResourceNotFoundException")) {
          throw removeError;
        }
      }
      await addPermission();
    }
  };

  const ensureUrlPermission = async () => {
    const urlStatementId = sanitizeIdentifier(`runfabric-url-${stage}`).slice(0, 100) || "runfabric-url";
    const invokeStatementId =
      sanitizeIdentifier(`runfabric-url-invoke-${stage}`).slice(0, 100) || "runfabric-url-invoke";

    await upsertPermission({
      statementId: urlStatementId,
      action: "lambda:InvokeFunctionUrl",
      functionUrlAuthType: "NONE"
    });
    await upsertPermission({
      statementId: invokeStatementId,
      action: "lambda:InvokeFunction",
      invokedViaFunctionUrl: true
    });
  };

  try {
    const existing = await client.send(new GetFunctionUrlConfigCommand({ FunctionName: functionName }));
    const existingUrl = readString(existing.FunctionUrl);
    if (existingUrl) {
      await ensureUrlPermission();
      return existingUrl;
    }
  } catch (error) {
    if (!isAwsErrorNamed(error, "ResourceNotFoundException")) {
      throw error;
    }
  }

  const created = await client.send(
    new CreateFunctionUrlConfigCommand({
      FunctionName: functionName,
      AuthType: "NONE"
    })
  );
  const createdUrl = readString(created.FunctionUrl);
  if (!createdUrl) {
    throw new Error("aws-lambda internal deploy did not return a function URL");
  }
  await ensureUrlPermission();

  return createdUrl;
}

async function deployWithAwsSdk(
  project: ProjectConfig,
  plan: DeployPlan,
  projectDir: string,
  stage: string,
  region: string,
  deployMetadata: ReturnType<typeof createAwsDeployMetadata>
): Promise<{ endpoint: string; rawResponse: Record<string, unknown>; resource: Record<string, unknown> }> {
  const roleArn = resolveRoleArn(project);
  if (!roleArn) {
    throw new Error(
      "aws-lambda internal deploy requires a role ARN. Set extensions.aws-lambda.roleArn or RUNFABRIC_AWS_LAMBDA_ROLE_ARN."
    );
  }

  const functionName = resolveFunctionName(project, stage);
  const runtime = resolveLambdaRuntime(project);
  const handler = resolveLambdaHandler(projectDir, plan);
  const zipBuffer = await createZipFromArtifact(projectDir, plan);
  const timeout = resolveLambdaTimeout(project);
  const memory = resolveLambdaMemory(project);
  const client = new LambdaClient({ region });

  const environmentVariables = Object.keys(deployMetadata.functionEnv).length > 0
    ? deployMetadata.functionEnv
    : undefined;

  let functionArn: string | undefined;
  let existingFunction = false;

  try {
    const existing = await client.send(new GetFunctionCommand({ FunctionName: functionName }));
    existingFunction = true;
    functionArn = readString(existing.Configuration?.FunctionArn);
  } catch (error) {
    if (!isAwsErrorNamed(error, "ResourceNotFoundException")) {
      throw error;
    }
  }

  if (existingFunction) {
    await waitUntilFunctionUpdatedV2(
      {
        client,
        maxWaitTime: 120,
        minDelay: 1,
        maxDelay: 5
      },
      { FunctionName: functionName }
    );

    const codeResult = await client.send(
      new UpdateFunctionCodeCommand({
        FunctionName: functionName,
        ZipFile: zipBuffer,
        Publish: true
      })
    );
    functionArn = readString(codeResult.FunctionArn) || functionArn;

    await waitUntilFunctionUpdatedV2(
      {
        client,
        maxWaitTime: 120,
        minDelay: 1,
        maxDelay: 5
      },
      { FunctionName: functionName }
    );

    await client.send(
      new UpdateFunctionConfigurationCommand({
        FunctionName: functionName,
        Runtime: runtime,
        Handler: handler,
        ...(environmentVariables ? { Environment: { Variables: environmentVariables } } : {}),
        ...(typeof timeout === "number" ? { Timeout: timeout } : {}),
        ...(typeof memory === "number" ? { MemorySize: memory } : {})
      })
    );

    await waitUntilFunctionUpdatedV2(
      {
        client,
        maxWaitTime: 120,
        minDelay: 1,
        maxDelay: 5
      },
      { FunctionName: functionName }
    );
  } else {
    const createResult = await client.send(
      new CreateFunctionCommand({
        FunctionName: functionName,
        Runtime: runtime,
        Role: roleArn,
        Handler: handler,
        Code: {
          ZipFile: zipBuffer
        },
        Publish: true,
        ...(environmentVariables ? { Environment: { Variables: environmentVariables } } : {}),
        ...(typeof timeout === "number" ? { Timeout: timeout } : {}),
        ...(typeof memory === "number" ? { MemorySize: memory } : {})
      })
    );
    functionArn = readString(createResult.FunctionArn) || functionArn;

    await waitUntilFunctionActiveV2(
      {
        client,
        maxWaitTime: 120,
        minDelay: 1,
        maxDelay: 5
      },
      { FunctionName: functionName }
    );
  }

  const functionUrl = await ensureFunctionUrl(client, functionName, stage);
  const rawResponse: Record<string, unknown> = {
    FunctionName: functionName,
    FunctionArn: functionArn,
    FunctionUrl: functionUrl,
    Runtime: runtime,
    Handler: handler,
    deployment: "internal-api",
    region
  };

  return {
    endpoint: functionUrl,
    rawResponse,
    resource: {
      FunctionName: functionName,
      FunctionArn: functionArn,
      Runtime: runtime,
      Handler: handler,
      Region: region
    }
  };
}

async function destroyWithAwsSdk(
  project: ProjectConfig,
  stage: string,
  region: string
): Promise<void> {
  const functionName = resolveFunctionName(project, stage);
  const client = new LambdaClient({ region });

  try {
    await client.send(new DeleteFunctionUrlConfigCommand({ FunctionName: functionName }));
  } catch (error) {
    if (!isAwsErrorNamed(error, "ResourceNotFoundException")) {
      throw error;
    }
  }

  try {
    await client.send(new DeleteFunctionCommand({ FunctionName: functionName }));
  } catch (error) {
    if (!isAwsErrorNamed(error, "ResourceNotFoundException")) {
      throw error;
    }
  }
}

export function createAwsLambdaProvider(options: AwsProviderOptions): ProviderAdapter {
  return {
    name: "aws-lambda",
    getCapabilities() {
      return awsLambdaCapabilities;
    },
    getCredentialSchema() {
      return awsLambdaCredentialSchema;
    },
    async validate(project: ProjectConfig): Promise<ValidationResult> {
      const warnings: string[] = [];
      const errors: string[] = [];

      if (!project.providers.includes("aws-lambda")) {
        warnings.push("project does not target aws-lambda in providers");
      }

      const isSupported = project.triggers.some((trigger) =>
        [
          TriggerEnum.Http,
          TriggerEnum.Cron,
          TriggerEnum.Queue,
          TriggerEnum.Storage,
          TriggerEnum.EventBridge
        ].includes(
          trigger.type
        ));

      if (!isSupported) {
        warnings.push("aws-lambda provider has no supported triggers configured");
      }

      errors.push(...missingRequiredCredentialErrors(awsLambdaCredentialSchema));

      return {
        ok: errors.length === 0,
        warnings,
        errors
      };
    },
    async provisionResources(project: ProjectConfig) {
      const region = resolveRegion(project);
      return {
        provider: "aws-lambda",
        resourceAddresses: collectResourceAddresses(project, region)
      };
    },
    async deployWorkflows(project: ProjectConfig) {
      const stage = resolveStage(project);
      const region = resolveRegion(project);
      return {
        provider: "aws-lambda",
        workflowAddresses: collectWorkflowAddresses(project, region, stage)
      };
    },
    async materializeSecrets(project: ProjectConfig) {
      const region = resolveRegion(project);
      return {
        provider: "aws-lambda",
        secretReferences: collectSecretReferences(project, region)
      };
    },
    async planBuild(): Promise<BuildPlan> {
      return {
        provider: "aws-lambda",
        steps: ["validate config", "prepare aws-lambda artifact manifest"]
      };
    },
    async build(_project: ProjectConfig, plan: BuildPlan): Promise<BuildResult> {
      return {
        artifacts: plan.steps.map((step, index) => ({
          provider: "aws-lambda",
          entry: step,
          outputPath: `aws-lambda-step-${index + 1}`
        }))
      };
    },
    async planDeploy(_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> {
      return {
        provider: "aws-lambda",
        artifactPath: artifact.entry,
        artifactManifestPath: artifact.outputPath,
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      const stage = resolveStage(project);
      const region = resolveRegion(project);

      const deploymentId = createDeploymentId("aws-lambda", project.service, stage);
      let endpoint = `https://${project.service}.execute-api.${region}.amazonaws.com/${stage}`;
      let mode: "simulated" | "cli" | "api" = "simulated";
      let rawResponse: unknown;
      let resource: Record<string, unknown> | undefined;
      const deployMetadata = createAwsDeployMetadata(project, region, stage);

      if (isRealDeployModeEnabled("RUNFABRIC_AWS_REAL_DEPLOY")) {
        const deployCommand = process.env.RUNFABRIC_AWS_DEPLOY_CMD;
        if (deployCommand) {
          rawResponse = await runJsonCommand(deployCommand, {
            cwd: options.projectDir,
            env: {
              RUNFABRIC_SERVICE: project.service,
              RUNFABRIC_STAGE: stage,
              RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
              RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath,
              RUNFABRIC_AWS_QUEUE_EVENT_SOURCES_JSON: JSON.stringify(deployMetadata.queueEventSources),
              RUNFABRIC_AWS_STORAGE_EVENTS_JSON: JSON.stringify(deployMetadata.storageEvents),
              RUNFABRIC_AWS_EVENTBRIDGE_RULES_JSON: JSON.stringify(deployMetadata.eventBridgeRules),
              RUNFABRIC_AWS_IAM_ROLE_STATEMENTS_JSON: JSON.stringify(deployMetadata.iamRoleStatements),
              RUNFABRIC_FUNCTION_ENV_JSON: JSON.stringify(deployMetadata.functionEnv),
              RUNFABRIC_AWS_RESOURCE_ADDRESSES_JSON: JSON.stringify(deployMetadata.resourceAddresses),
              RUNFABRIC_AWS_WORKFLOW_ADDRESSES_JSON: JSON.stringify(deployMetadata.workflowAddresses),
              RUNFABRIC_AWS_SECRET_REFERENCES_JSON: JSON.stringify(deployMetadata.secretReferences)
            }
          });
          const parsedEndpoint = endpointFromAwsResponse(rawResponse);
          if (!parsedEndpoint) {
            throw new Error("aws-lambda deploy response does not include an endpoint/function URL");
          }
          endpoint = parsedEndpoint;
          resource = awsResourceMetadata(rawResponse);
          mode = "cli";
        } else {
          const internalDeploy = await deployWithAwsSdk(
            project,
            plan,
            options.projectDir,
            stage,
            region,
            deployMetadata
          );
          endpoint = internalDeploy.endpoint;
          rawResponse = internalDeploy.rawResponse;
          resource = internalDeploy.resource;
          mode = "api";
        }
      }

      resource = {
        ...(resource || {}),
        queueEventSources: deployMetadata.queueEventSources,
        storageEvents: deployMetadata.storageEvents,
        eventBridgeRules: deployMetadata.eventBridgeRules,
        iamRoleStatements: deployMetadata.iamRoleStatements,
        functionEnvKeys: Object.keys(deployMetadata.functionEnv),
        resourceAddresses: deployMetadata.resourceAddresses,
        workflowAddresses: deployMetadata.workflowAddresses,
        secretReferences: deployMetadata.secretReferences
      };

      await writeDeploymentReceipt(options.projectDir, "aws-lambda", {
        provider: "aws-lambda",
        service: project.service,
        stage,
        deploymentId,
        endpoint,
        mode,
        executedSteps: plan.steps,
        artifactPath: plan.artifactPath,
        artifactManifestPath: plan.artifactManifestPath,
        resource,
        rawResponse,
        createdAt: new Date().toISOString()
      });
      await appendProviderLog(
        options.projectDir,
        "aws-lambda",
        `deploy deploymentId=${deploymentId} mode=${mode} endpoint=${endpoint}`
      );

      return {
        provider: "aws-lambda",
        endpoint,
        resourceAddresses: deployMetadata.resourceAddresses,
        workflowAddresses: deployMetadata.workflowAddresses,
        secretReferences: deployMetadata.secretReferences
      };
    },
    async invoke(input) {
      return invokeProviderViaDeployedEndpoint(options.projectDir, "aws-lambda", input);
    },
    async logs(input) {
      return buildProviderLogsFromLocalArtifacts(options.projectDir, "aws-lambda", input);
    },
    async destroy(project: ProjectConfig) {
      const stage = resolveStage(project);
      const region = resolveRegion(project);
      const realDeployModeEnabled = isRealDeployModeEnabled("RUNFABRIC_AWS_REAL_DEPLOY");
      if (realDeployModeEnabled) {
        if (process.env.RUNFABRIC_AWS_DESTROY_CMD) {
          const result = await runShellCommand(process.env.RUNFABRIC_AWS_DESTROY_CMD, {
            cwd: options.projectDir,
            env: {
              RUNFABRIC_SERVICE: project.service,
              RUNFABRIC_STAGE: stage
            }
          });
          if (result.code !== 0) {
            throw new Error(result.stderr || result.stdout || "aws-lambda destroy command failed");
          }
        } else {
          await destroyWithAwsSdk(project, stage, region);
        }
      } else {
        const receipt = await readDeploymentReceipt(options.projectDir, "aws-lambda");
        if (receipt && receipt.mode !== "simulated") {
          throw new Error(
            `aws-lambda destroy skipped cloud deletion because real deploy mode is disabled. Last deployment mode=${receipt.mode}. Set RUNFABRIC_AWS_REAL_DEPLOY=1 and rerun runfabric remove.`
          );
        }
      }

      await appendProviderLog(options.projectDir, "aws-lambda", "destroy local artifacts");
      await destroyProviderArtifacts(options.projectDir, "aws-lambda");
    }
  };
}
