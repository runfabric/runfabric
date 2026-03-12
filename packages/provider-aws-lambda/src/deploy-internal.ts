import { readdir, readFile } from "node:fs/promises";
import { basename, dirname, extname, relative, resolve } from "node:path";
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
import type { DeployPlan } from "@runfabric/core";
import JSZip from "jszip";

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function sanitizeIdentifier(value: string): string {
  return value.replace(/[^a-zA-Z0-9-_./]/g, "-");
}

function resolveProjectPath(projectDir: string, filePath: string | undefined): string | undefined {
  if (!filePath) {
    return undefined;
  }
  return resolve(projectDir, filePath);
}

async function collectFilesRecursively(rootDir: string): Promise<string[]> {
  const entries = await readdir(rootDir, { withFileTypes: true });
  const files: string[] = [];

  for (const entry of entries) {
    const absolute = resolve(rootDir, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await collectFilesRecursively(absolute)));
      continue;
    }
    if (entry.isFile()) {
      files.push(absolute);
    }
  }

  return files;
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
    zip.file(relativePath, await readFile(absolutePath));
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
  return Boolean(error) && typeof error === "object" && typeof (error as { name?: unknown }).name === "string"
    ? names.includes((error as { name: string }).name)
    : false;
}

async function upsertPermission(
  client: LambdaClient,
  functionName: string,
  input: {
    statementId: string;
    action: string;
    functionUrlAuthType?: "NONE" | "AWS_IAM";
    invokedViaFunctionUrl?: boolean;
  }
): Promise<void> {
  const addPermission = async () => {
    await client.send(
      new AddPermissionCommand({
        FunctionName: functionName,
        StatementId: input.statementId,
        Action: input.action,
        Principal: "*",
        ...(input.functionUrlAuthType ? { FunctionUrlAuthType: input.functionUrlAuthType } : {}),
        ...(typeof input.invokedViaFunctionUrl === "boolean"
          ? { InvokedViaFunctionUrl: input.invokedViaFunctionUrl }
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
      await client.send(new RemovePermissionCommand({ FunctionName: functionName, StatementId: input.statementId }));
    } catch (removeError) {
      if (!isAwsErrorNamed(removeError, "ResourceNotFoundException")) {
        throw removeError;
      }
    }
    await addPermission();
  }
}

async function ensureUrlPermissions(client: LambdaClient, functionName: string, stage: string): Promise<void> {
  const urlStatementId = sanitizeIdentifier(`runfabric-url-${stage}`).slice(0, 100) || "runfabric-url";
  const invokeStatementId =
    sanitizeIdentifier(`runfabric-url-invoke-${stage}`).slice(0, 100) || "runfabric-url-invoke";
  await upsertPermission(client, functionName, {
    statementId: urlStatementId,
    action: "lambda:InvokeFunctionUrl",
    functionUrlAuthType: "NONE"
  });
  await upsertPermission(client, functionName, {
    statementId: invokeStatementId,
    action: "lambda:InvokeFunction",
    invokedViaFunctionUrl: true
  });
}

async function ensureFunctionUrl(client: LambdaClient, functionName: string, stage: string): Promise<string> {
  try {
    const existing = await client.send(new GetFunctionUrlConfigCommand({ FunctionName: functionName }));
    const existingUrl = readString(existing.FunctionUrl);
    if (existingUrl) {
      await ensureUrlPermissions(client, functionName, stage);
      return existingUrl;
    }
  } catch (error) {
    if (!isAwsErrorNamed(error, "ResourceNotFoundException")) {
      throw error;
    }
  }

  const created = await client.send(
    new CreateFunctionUrlConfigCommand({ FunctionName: functionName, AuthType: "NONE" })
  );
  const createdUrl = readString(created.FunctionUrl);
  if (!createdUrl) {
    throw new Error("aws-lambda internal deploy did not return a function URL");
  }
  await ensureUrlPermissions(client, functionName, stage);
  return createdUrl;
}

async function waitForFunctionUpdated(client: LambdaClient, functionName: string): Promise<void> {
  await waitUntilFunctionUpdatedV2(
    { client, maxWaitTime: 120, minDelay: 1, maxDelay: 5 },
    { FunctionName: functionName }
  );
}

async function updateExistingFunction(input: {
  client: LambdaClient;
  functionName: string;
  zipBuffer: Buffer;
  runtime: Runtime;
  handler: string;
  functionEnv: Record<string, string>;
  timeout?: number;
  memory?: number;
}): Promise<string | undefined> {
  await waitForFunctionUpdated(input.client, input.functionName);
  const codeResult = await input.client.send(
    new UpdateFunctionCodeCommand({
      FunctionName: input.functionName,
      ZipFile: input.zipBuffer,
      Publish: true
    })
  );
  const functionArn = readString(codeResult.FunctionArn);

  await waitForFunctionUpdated(input.client, input.functionName);
  await input.client.send(
    new UpdateFunctionConfigurationCommand({
      FunctionName: input.functionName,
      Runtime: input.runtime,
      Handler: input.handler,
      ...(Object.keys(input.functionEnv).length > 0 ? { Environment: { Variables: input.functionEnv } } : {}),
      ...(typeof input.timeout === "number" ? { Timeout: input.timeout } : {}),
      ...(typeof input.memory === "number" ? { MemorySize: input.memory } : {})
    })
  );
  await waitForFunctionUpdated(input.client, input.functionName);
  return functionArn;
}

async function createNewFunction(input: {
  client: LambdaClient;
  functionName: string;
  runtime: Runtime;
  roleArn: string;
  handler: string;
  zipBuffer: Buffer;
  functionEnv: Record<string, string>;
  timeout?: number;
  memory?: number;
}): Promise<string | undefined> {
  const createResult = await input.client.send(
    new CreateFunctionCommand({
      FunctionName: input.functionName,
      Runtime: input.runtime,
      Role: input.roleArn,
      Handler: input.handler,
      Code: { ZipFile: input.zipBuffer },
      Publish: true,
      ...(Object.keys(input.functionEnv).length > 0 ? { Environment: { Variables: input.functionEnv } } : {}),
      ...(typeof input.timeout === "number" ? { Timeout: input.timeout } : {}),
      ...(typeof input.memory === "number" ? { MemorySize: input.memory } : {})
    })
  );

  await waitUntilFunctionActiveV2(
    { client: input.client, maxWaitTime: 120, minDelay: 1, maxDelay: 5 },
    { FunctionName: input.functionName }
  );
  return readString(createResult.FunctionArn);
}

export async function deployWithAwsSdk(input: {
  projectDir: string;
  plan: DeployPlan;
  stage: string;
  region: string;
  roleArn: string;
  functionName: string;
  runtime: Runtime;
  timeout?: number;
  memory?: number;
  functionEnv: Record<string, string>;
}): Promise<{ endpoint: string; rawResponse: Record<string, unknown>; resource: Record<string, unknown> }> {
  const client = new LambdaClient({ region: input.region });
  const handler = resolveLambdaHandler(input.projectDir, input.plan);
  const zipBuffer = await createZipFromArtifact(input.projectDir, input.plan);
  const functionArn = await deployOrUpdateFunction({
    client,
    functionName: input.functionName,
    runtime: input.runtime,
    roleArn: input.roleArn,
    handler,
    zipBuffer,
    functionEnv: input.functionEnv,
    timeout: input.timeout,
    memory: input.memory
  });
  const functionUrl = await ensureFunctionUrl(client, input.functionName, input.stage);

  return buildDeployResponse({
    functionName: input.functionName,
    functionArn,
    functionUrl,
    runtime: input.runtime,
    handler,
    region: input.region
  });
}

async function deployOrUpdateFunction(input: {
  client: LambdaClient;
  functionName: string;
  runtime: Runtime;
  roleArn: string;
  handler: string;
  zipBuffer: Buffer;
  functionEnv: Record<string, string>;
  timeout?: number;
  memory?: number;
}): Promise<string | undefined> {
  try {
    const existing = await input.client.send(new GetFunctionCommand({ FunctionName: input.functionName }));
    const existingArn = readString(existing.Configuration?.FunctionArn);
    const updatedArn = await updateExistingFunction({
      client: input.client,
      functionName: input.functionName,
      zipBuffer: input.zipBuffer,
      runtime: input.runtime,
      handler: input.handler,
      functionEnv: input.functionEnv,
      timeout: input.timeout,
      memory: input.memory
    });
    return updatedArn || existingArn;
  } catch (error) {
    if (!isAwsErrorNamed(error, "ResourceNotFoundException")) {
      throw error;
    }
    return createNewFunction({
      client: input.client,
      functionName: input.functionName,
      runtime: input.runtime,
      roleArn: input.roleArn,
      handler: input.handler,
      zipBuffer: input.zipBuffer,
      functionEnv: input.functionEnv,
      timeout: input.timeout,
      memory: input.memory
    });
  }
}

function buildDeployResponse(input: {
  functionName: string;
  functionArn: string | undefined;
  functionUrl: string;
  runtime: Runtime;
  handler: string;
  region: string;
}): { endpoint: string; rawResponse: Record<string, unknown>; resource: Record<string, unknown> } {
  const rawResponse: Record<string, unknown> = {
    FunctionName: input.functionName,
    FunctionArn: input.functionArn,
    FunctionUrl: input.functionUrl,
    Runtime: input.runtime,
    Handler: input.handler,
    deployment: "internal-api",
    region: input.region
  };
  return {
    endpoint: input.functionUrl,
    rawResponse,
    resource: {
      FunctionName: input.functionName,
      FunctionArn: input.functionArn,
      Runtime: input.runtime,
      Handler: input.handler,
      Region: input.region
    }
  };
}

export async function destroyWithAwsSdk(functionName: string, region: string): Promise<void> {
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
