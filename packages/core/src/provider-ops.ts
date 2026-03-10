import { spawn } from "node:child_process";
import { randomUUID } from "node:crypto";
import { appendFile, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import type {
  InvokeInput,
  InvokeResult,
  LogsInput,
  LogsResult
} from "./provider";
import type { ProviderCredentialSchema } from "./credentials";

export type DeploymentMode = "simulated" | "api" | "cli";

export interface DeploymentReceipt {
  provider: string;
  service: string;
  stage: string;
  deploymentId: string;
  endpoint?: string;
  mode: DeploymentMode;
  executedSteps: string[];
  artifactPath?: string;
  artifactManifestPath?: string;
  resource?: Record<string, unknown>;
  rawResponse?: unknown;
  createdAt: string;
}

export interface ProviderDeployPaths {
  deployDir: string;
  receiptPath: string;
  logPath: string;
}

export interface CommandResult {
  command: string;
  code: number;
  stdout: string;
  stderr: string;
}

function toBooleanFlag(value: string | undefined): boolean {
  if (!value) {
    return false;
  }
  const normalized = value.trim().toLowerCase();
  return normalized === "1" || normalized === "true" || normalized === "yes" || normalized === "on";
}

export function isRealDeployModeEnabled(providerFlagEnv: string): boolean {
  return toBooleanFlag(process.env.RUNFABRIC_REAL_DEPLOY) || toBooleanFlag(process.env[providerFlagEnv]);
}

export function providerDeployPaths(projectDir: string, provider: string): ProviderDeployPaths {
  const deployDir = resolve(projectDir, ".runfabric", "deploy", provider);
  return {
    deployDir,
    receiptPath: join(deployDir, "deployment.json"),
    logPath: join(deployDir, "events.log")
  };
}

export function missingRequiredCredentialErrors(
  schema: ProviderCredentialSchema,
  env: Record<string, string | undefined> = process.env
): string[] {
  const errors: string[] = [];
  for (const field of schema.fields) {
    if (field.required === false) {
      continue;
    }

    const envValue = env[field.env];
    if (typeof envValue !== "string" || envValue.trim().length === 0) {
      errors.push(`missing credential env ${field.env} (${field.description})`);
    }
  }
  return errors;
}

export function createDeploymentId(provider: string, service: string, stage: string): string {
  const suffix = randomUUID().split("-")[0];
  return `${provider}-${service}-${stage}-${suffix}`.replace(/[^a-zA-Z0-9-_]/g, "-");
}

export async function writeDeploymentReceipt(
  projectDir: string,
  provider: string,
  receipt: DeploymentReceipt
): Promise<void> {
  const paths = providerDeployPaths(projectDir, provider);
  await mkdir(paths.deployDir, { recursive: true });
  await writeFile(paths.receiptPath, JSON.stringify(receipt, null, 2), "utf8");
}

export async function readDeploymentReceipt(
  projectDir: string,
  provider: string
): Promise<DeploymentReceipt | null> {
  const paths = providerDeployPaths(projectDir, provider);
  try {
    const content = await readFile(paths.receiptPath, "utf8");
    return JSON.parse(content) as DeploymentReceipt;
  } catch (error) {
    const code = (error as NodeJS.ErrnoException).code;
    if (code === "ENOENT") {
      return null;
    }
    throw error;
  }
}

export async function appendProviderLog(
  projectDir: string,
  provider: string,
  message: string
): Promise<void> {
  const paths = providerDeployPaths(projectDir, provider);
  await mkdir(paths.deployDir, { recursive: true });
  await appendFile(paths.logPath, `${new Date().toISOString()} ${message}\n`, "utf8");
}

export async function readProviderLogLines(
  projectDir: string,
  provider: string,
  since?: string
): Promise<string[]> {
  const paths = providerDeployPaths(projectDir, provider);
  let content: string;
  try {
    content = await readFile(paths.logPath, "utf8");
  } catch (error) {
    const code = (error as NodeJS.ErrnoException).code;
    if (code === "ENOENT") {
      return [];
    }
    throw error;
  }

  const threshold = since ? Date.parse(since) : Number.NaN;
  const lines = content
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0);

  if (Number.isNaN(threshold)) {
    return lines;
  }

  return lines.filter((line) => {
    const firstSpace = line.indexOf(" ");
    if (firstSpace <= 0) {
      return true;
    }
    const timestamp = Date.parse(line.slice(0, firstSpace));
    if (Number.isNaN(timestamp)) {
      return true;
    }
    return timestamp >= threshold;
  });
}

export async function buildProviderLogsFromLocalArtifacts(
  projectDir: string,
  provider: string,
  input: LogsInput
): Promise<LogsResult> {
  const lines = await readProviderLogLines(projectDir, provider, input.since);
  if (lines.length > 0) {
    return { lines };
  }

  const receipt = await readDeploymentReceipt(projectDir, provider);
  if (!receipt) {
    return {
      lines: [`${provider}: no deployment receipt/logs found`]
    };
  }

  return {
    lines: [
      `${provider}: deploymentId=${receipt.deploymentId}`,
      `${provider}: mode=${receipt.mode}`,
      `${provider}: endpoint=${receipt.endpoint || "n/a"}`,
      `${provider}: createdAt=${receipt.createdAt}`
    ]
  };
}

function resolvePayload(input: InvokeInput): { method: string; body?: string; contentType?: string } {
  if (!input.payload) {
    return { method: "GET" };
  }

  const trimmed = input.payload.trim();
  if (trimmed.length === 0) {
    return { method: "POST", body: "", contentType: "text/plain" };
  }

  try {
    JSON.parse(trimmed);
    return {
      method: "POST",
      body: trimmed,
      contentType: "application/json"
    };
  } catch {
    return {
      method: "POST",
      body: input.payload,
      contentType: "text/plain"
    };
  }
}

export async function invokeProviderViaDeployedEndpoint(
  projectDir: string,
  provider: string,
  input: InvokeInput
): Promise<InvokeResult> {
  const receipt = await readDeploymentReceipt(projectDir, provider);
  if (!receipt?.endpoint) {
    return {
      statusCode: 404,
      body: JSON.stringify({
        error: `${provider} deployment endpoint is not available`,
        hint: "run deploy first"
      })
    };
  }

  const payload = resolvePayload(input);
  const headers: Record<string, string> = {
    "x-runfabric-provider": provider
  };
  if (payload.contentType) {
    headers["content-type"] = payload.contentType;
  }

  try {
    const response = await fetch(receipt.endpoint, {
      method: payload.method,
      headers,
      body: payload.body,
      signal: AbortSignal.timeout(10_000)
    });
    const bodyText = await response.text();
    await appendProviderLog(
      projectDir,
      provider,
      `invoke endpoint=${receipt.endpoint} status=${response.status}`
    );
    return {
      statusCode: response.status,
      body: bodyText
    };
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    await appendProviderLog(projectDir, provider, `invoke failed error=${message}`);
    return {
      statusCode: 502,
      body: JSON.stringify({
        error: "invoke failed",
        message
      })
    };
  }
}

export async function destroyProviderArtifacts(projectDir: string, provider: string): Promise<void> {
  const paths = providerDeployPaths(projectDir, provider);
  await rm(paths.deployDir, { recursive: true, force: true });
}

export async function runShellCommand(
  command: string,
  options?: { cwd?: string; env?: Record<string, string | undefined> }
): Promise<CommandResult> {
  return new Promise((resolvePromise, rejectPromise) => {
    const child = spawn("sh", ["-lc", command], {
      cwd: options?.cwd,
      env: {
        ...process.env,
        ...(options?.env || {})
      },
      stdio: ["ignore", "pipe", "pipe"]
    });

    let stdout = "";
    let stderr = "";

    child.stdout.on("data", (chunk: Buffer | string) => {
      stdout += String(chunk);
    });
    child.stderr.on("data", (chunk: Buffer | string) => {
      stderr += String(chunk);
    });
    child.on("error", (error) => {
      rejectPromise(error);
    });
    child.on("close", (code) => {
      resolvePromise({
        command,
        code: code ?? 1,
        stdout,
        stderr
      });
    });
  });
}

export async function runJsonCommand(
  command: string,
  options?: { cwd?: string; env?: Record<string, string | undefined> }
): Promise<unknown> {
  const result = await runShellCommand(command, options);
  if (result.code !== 0) {
    throw new Error(
      `command failed (${result.code}): ${command}\n${result.stderr || result.stdout || "no output"}`
    );
  }

  const output = result.stdout.trim();
  if (!output) {
    throw new Error(`command produced empty output: ${command}`);
  }

  try {
    return JSON.parse(output) as unknown;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`command output is not valid JSON: ${message}`);
  }
}
