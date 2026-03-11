import { spawn } from "node:child_process";
import { randomUUID } from "node:crypto";
import { appendFile, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import type {
  InvokeInput,
  InvokeResult,
  MetricsInput,
  MetricsResult,
  LogsInput,
  LogsResult,
  TraceRecord,
  TracesInput,
  TracesResult
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
  correlation?: {
    deploymentId: string;
    deployTraceId: string;
    latestInvokeId?: string;
    latestInvokeAt?: string;
  };
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
  const normalized: DeploymentReceipt = {
    ...receipt,
    correlation: {
      deploymentId: receipt.deploymentId,
      deployTraceId: receipt.correlation?.deployTraceId || `deploy-${receipt.deploymentId}`,
      latestInvokeId: receipt.correlation?.latestInvokeId,
      latestInvokeAt: receipt.correlation?.latestInvokeAt
    }
  };
  await writeFile(paths.receiptPath, JSON.stringify(normalized, null, 2), "utf8");
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

function parseLogLineToTrace(provider: string, line: string): TraceRecord {
  const firstSpace = line.indexOf(" ");
  const timestamp = firstSpace > 0 ? line.slice(0, firstSpace) : new Date().toISOString();
  const message = firstSpace > 0 ? line.slice(firstSpace + 1) : line;

  const deploymentMatch = message.match(/deploymentId=([^\s]+)/);
  const invokeMatch = message.match(/invokeId=([^\s]+)/);
  const correlationMatch = message.match(/correlationId=([^\s]+)/);

  return {
    timestamp,
    provider,
    message,
    deploymentId: deploymentMatch?.[1],
    invokeId: invokeMatch?.[1],
    correlationId: correlationMatch?.[1]
  };
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

export async function buildProviderTracesFromLocalArtifacts(
  projectDir: string,
  provider: string,
  input: TracesInput
): Promise<TracesResult> {
  const logs = await readProviderLogLines(projectDir, provider, input.since);
  let traces = logs.map((line) => parseLogLineToTrace(provider, line));

  if (input.correlationId) {
    traces = traces.filter((trace) =>
      trace.message.includes(input.correlationId || "")
    );
  }

  if (typeof input.limit === "number" && Number.isFinite(input.limit) && input.limit > 0) {
    traces = traces.slice(-Math.floor(input.limit));
  }

  if (traces.length > 0) {
    return { traces };
  }

  const receipt = await readDeploymentReceipt(projectDir, provider);
  if (!receipt) {
    return { traces: [] };
  }

  return {
    traces: [
      {
        timestamp: receipt.createdAt,
        provider,
        message: `deploy receipt deploymentId=${receipt.deploymentId} mode=${receipt.mode}`,
        deploymentId: receipt.deploymentId,
        correlationId: receipt.correlation?.deployTraceId
      }
    ]
  };
}

export async function buildProviderMetricsFromLocalArtifacts(
  projectDir: string,
  provider: string,
  input: MetricsInput
): Promise<MetricsResult> {
  const logs = await readProviderLogLines(projectDir, provider, input.since);
  const invokeLines = logs.filter((line) => line.includes(" invoke "));
  const invokeFailures = logs.filter((line) => line.includes("invoke failed")).length;
  const deployLines = logs.filter((line) => line.includes("deploy deploymentId=")).length;

  const receipt = await readDeploymentReceipt(projectDir, provider);
  const now = Date.now();
  const deployAgeSeconds =
    receipt && !Number.isNaN(Date.parse(receipt.createdAt))
      ? Math.max(0, Math.floor((now - Date.parse(receipt.createdAt)) / 1000))
      : NaN;

  const metrics: MetricsResult["metrics"] = [
    { name: "log_lines_total", value: logs.length, unit: "count" },
    { name: "invoke_total", value: invokeLines.length, unit: "count" },
    { name: "invoke_failures_total", value: invokeFailures, unit: "count" },
    { name: "deploy_total", value: deployLines, unit: "count" }
  ];

  if (!Number.isNaN(deployAgeSeconds)) {
    metrics.push({
      name: "seconds_since_last_deploy",
      value: deployAgeSeconds,
      unit: "seconds"
    });
  }

  return { metrics };
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
  const invokeId = randomUUID();
  const correlationId = `invoke-${invokeId}`;
  const headers: Record<string, string> = {
    "x-runfabric-provider": provider
  };
  headers["x-runfabric-deployment-id"] = receipt.deploymentId;
  headers["x-runfabric-invoke-id"] = invokeId;
  headers["x-runfabric-correlation-id"] = correlationId;
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
      `invoke deploymentId=${receipt.deploymentId} invokeId=${invokeId} correlationId=${correlationId} endpoint=${receipt.endpoint} status=${response.status}`
    );
    await writeDeploymentReceipt(projectDir, provider, {
      ...receipt,
      correlation: {
        deploymentId: receipt.deploymentId,
        deployTraceId: receipt.correlation?.deployTraceId || `deploy-${receipt.deploymentId}`,
        latestInvokeId: invokeId,
        latestInvokeAt: new Date().toISOString()
      }
    });
    return {
      statusCode: response.status,
      body: bodyText,
      correlation: {
        deploymentId: receipt.deploymentId,
        invokeId
      }
    };
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    await appendProviderLog(
      projectDir,
      provider,
      `invoke failed deploymentId=${receipt.deploymentId} invokeId=${invokeId} correlationId=${correlationId} error=${message}`
    );
    await writeDeploymentReceipt(projectDir, provider, {
      ...receipt,
      correlation: {
        deploymentId: receipt.deploymentId,
        deployTraceId: receipt.correlation?.deployTraceId || `deploy-${receipt.deploymentId}`,
        latestInvokeId: invokeId,
        latestInvokeAt: new Date().toISOString()
      }
    });
    return {
      statusCode: 502,
      body: JSON.stringify({
        error: "invoke failed",
        message
      }),
      correlation: {
        deploymentId: receipt.deploymentId,
        invokeId
      }
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

function normalizeTraceRecord(raw: unknown, provider: string): TraceRecord | null {
  if (!raw || typeof raw !== "object") {
    return null;
  }

  const entry = raw as Record<string, unknown>;
  const timestamp =
    typeof entry.timestamp === "string" && entry.timestamp.trim().length > 0
      ? entry.timestamp
      : new Date().toISOString();
  const message =
    typeof entry.message === "string" && entry.message.trim().length > 0
      ? entry.message
      : JSON.stringify(entry);
  const deploymentId =
    typeof entry.deploymentId === "string" && entry.deploymentId.trim().length > 0
      ? entry.deploymentId
      : undefined;
  const invokeId =
    typeof entry.invokeId === "string" && entry.invokeId.trim().length > 0 ? entry.invokeId : undefined;
  const correlationId =
    typeof entry.correlationId === "string" && entry.correlationId.trim().length > 0
      ? entry.correlationId
      : undefined;

  return {
    timestamp,
    provider,
    message,
    deploymentId,
    invokeId,
    correlationId
  };
}

function normalizeMetricRecord(
  raw: unknown
): { name: string; value: number; unit?: string } | null {
  if (!raw || typeof raw !== "object") {
    return null;
  }
  const entry = raw as Record<string, unknown>;
  if (typeof entry.name !== "string" || entry.name.trim().length === 0) {
    return null;
  }
  if (typeof entry.value !== "number" || !Number.isFinite(entry.value)) {
    return null;
  }

  return {
    name: entry.name,
    value: entry.value,
    unit:
      typeof entry.unit === "string" && entry.unit.trim().length > 0
        ? entry.unit
        : undefined
  };
}

export function parseProviderTracesPayload(raw: unknown, provider: string): TracesResult {
  if (Array.isArray(raw)) {
    const traces = raw
      .map((entry) => normalizeTraceRecord(entry, provider))
      .filter((entry): entry is TraceRecord => entry !== null);
    return { traces };
  }

  if (raw && typeof raw === "object") {
    const payload = raw as Record<string, unknown>;
    if (Array.isArray(payload.traces)) {
      const traces = payload.traces
        .map((entry) => normalizeTraceRecord(entry, provider))
        .filter((entry): entry is TraceRecord => entry !== null);
      return { traces };
    }
  }

  throw new Error("trace command output must be { traces: [...] } or an array");
}

export function parseProviderMetricsPayload(raw: unknown): MetricsResult {
  if (Array.isArray(raw)) {
    const metrics = raw
      .map((entry) => normalizeMetricRecord(entry))
      .filter((entry): entry is { name: string; value: number; unit?: string } => entry !== null);
    return { metrics };
  }

  if (raw && typeof raw === "object") {
    const payload = raw as Record<string, unknown>;
    if (Array.isArray(payload.metrics)) {
      const metrics = payload.metrics
        .map((entry) => normalizeMetricRecord(entry))
        .filter((entry): entry is { name: string; value: number; unit?: string } => entry !== null);
      return { metrics };
    }
  }

  throw new Error("metrics command output must be { metrics: [...] } or an array");
}

export async function runProviderTracesCommand(
  command: string,
  provider: string,
  options?: { cwd?: string; env?: Record<string, string | undefined> }
): Promise<TracesResult> {
  const raw = await runJsonCommand(command, options);
  return parseProviderTracesPayload(raw, provider);
}

export async function runProviderMetricsCommand(
  command: string,
  options?: { cwd?: string; env?: Record<string, string | undefined> }
): Promise<MetricsResult> {
  const raw = await runJsonCommand(command, options);
  return parseProviderMetricsPayload(raw);
}
