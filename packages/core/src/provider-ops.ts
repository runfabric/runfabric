import { randomUUID } from "node:crypto";
import { createReadStream } from "node:fs";
import { appendFile, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { createInterface } from "node:readline";
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

function shouldIncludeLogLine(line: string, threshold: number): boolean {
  if (Number.isNaN(threshold)) {
    return true;
  }
  const firstSpace = line.indexOf(" ");
  if (firstSpace <= 0) {
    return true;
  }
  const timestamp = Date.parse(line.slice(0, firstSpace));
  if (Number.isNaN(timestamp)) {
    return true;
  }
  return timestamp >= threshold;
}

async function iterateProviderLogLines(
  projectDir: string,
  provider: string,
  since: string | undefined,
  onLine: (line: string) => void | Promise<void>
): Promise<void> {
  const paths = providerDeployPaths(projectDir, provider);
  const threshold = since ? Date.parse(since) : Number.NaN;
  const input = createReadStream(paths.logPath, { encoding: "utf8" });
  const reader = createInterface({
    input,
    crlfDelay: Infinity
  });
  try {
    for await (const rawLine of reader) {
      const line = rawLine.trim();
      if (!line || !shouldIncludeLogLine(line, threshold)) {
        continue;
      }
      await onLine(line);
    }
  } catch (error) {
    const code = (error as NodeJS.ErrnoException).code;
    if (code === "ENOENT") {
      return;
    }
    throw error;
  } finally {
    reader.close();
  }
}

function normalizeTraceLimit(limit: number | undefined): number {
  if (typeof limit !== "number" || !Number.isFinite(limit) || limit <= 0) {
    return 0;
  }
  return Math.floor(limit);
}

function createTraceCollector(limit: number): {
  add(trace: TraceRecord): void;
  finish(): TraceRecord[];
} {
  const traces: TraceRecord[] = [];
  let ringCursor = 0;
  let acceptedCount = 0;
  return {
    add(trace: TraceRecord): void {
      acceptedCount += 1;
      if (limit <= 0) {
        traces.push(trace);
        return;
      }
      if (traces.length < limit) {
        traces.push(trace);
        return;
      }
      traces[ringCursor] = trace;
      ringCursor = (ringCursor + 1) % limit;
    },
    finish(): TraceRecord[] {
      if (limit > 0 && acceptedCount > limit) {
        return [...traces.slice(ringCursor), ...traces.slice(0, ringCursor)];
      }
      return traces;
    }
  };
}

export async function readProviderLogLines(
  projectDir: string,
  provider: string,
  since?: string
): Promise<string[]> {
  const lines: string[] = [];
  await iterateProviderLogLines(projectDir, provider, since, (line) => {
    lines.push(line);
  });
  return lines;
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
  const traceCollector = createTraceCollector(normalizeTraceLimit(input.limit));

  await iterateProviderLogLines(projectDir, provider, input.since, (line) => {
    const trace = parseLogLineToTrace(provider, line);
    if (input.correlationId && !trace.message.includes(input.correlationId)) {
      return;
    }
    traceCollector.add(trace);
  });

  const traces = traceCollector.finish();
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
  let logLineCount = 0;
  let invokeCount = 0;
  let invokeFailures = 0;
  let deployCount = 0;

  await iterateProviderLogLines(projectDir, provider, input.since, (line) => {
    logLineCount += 1;
    if (line.includes(" invoke ")) {
      invokeCount += 1;
    }
    if (line.includes("invoke failed")) {
      invokeFailures += 1;
    }
    if (line.includes("deploy deploymentId=")) {
      deployCount += 1;
    }
  });

  const receipt = await readDeploymentReceipt(projectDir, provider);
  const now = Date.now();
  const deployAgeSeconds =
    receipt && !Number.isNaN(Date.parse(receipt.createdAt))
      ? Math.max(0, Math.floor((now - Date.parse(receipt.createdAt)) / 1000))
      : NaN;

  const metrics: MetricsResult["metrics"] = [
    { name: "log_lines_total", value: logLineCount, unit: "count" },
    { name: "invoke_total", value: invokeCount, unit: "count" },
    { name: "invoke_failures_total", value: invokeFailures, unit: "count" },
    { name: "deploy_total", value: deployCount, unit: "count" }
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

function missingEndpointInvokeResult(provider: string): InvokeResult {
  return {
    statusCode: 404,
    body: JSON.stringify({
      error: `${provider} deployment endpoint is not available`,
      hint: "run deploy first"
    })
  };
}

function buildInvokeHeaders(
  provider: string,
  deploymentId: string,
  invokeId: string,
  correlationId: string,
  contentType?: string
): Record<string, string> {
  const headers: Record<string, string> = {
    "x-runfabric-provider": provider,
    "x-runfabric-deployment-id": deploymentId,
    "x-runfabric-invoke-id": invokeId,
    "x-runfabric-correlation-id": correlationId
  };
  if (contentType) {
    headers["content-type"] = contentType;
  }
  return headers;
}

function invokeCorrelation(receipt: DeploymentReceipt, invokeId: string): NonNullable<InvokeResult["correlation"]> {
  return {
    deploymentId: receipt.deploymentId,
    invokeId
  };
}

async function persistInvokeCorrelation(
  projectDir: string,
  provider: string,
  receipt: DeploymentReceipt,
  invokeId: string
): Promise<void> {
  await writeDeploymentReceipt(projectDir, provider, {
    ...receipt,
    correlation: {
      deploymentId: receipt.deploymentId,
      deployTraceId: receipt.correlation?.deployTraceId || `deploy-${receipt.deploymentId}`,
      latestInvokeId: invokeId,
      latestInvokeAt: new Date().toISOString()
    }
  });
}

async function invokeEndpoint(
  endpoint: string,
  payload: { method: string; body?: string; contentType?: string },
  headers: Record<string, string>
): Promise<{ ok: true; statusCode: number; body: string } | { ok: false; message: string }> {
  try {
    const response = await fetch(endpoint, {
      method: payload.method,
      headers,
      body: payload.body,
      signal: AbortSignal.timeout(10_000)
    });
    return {
      ok: true,
      statusCode: response.status,
      body: await response.text()
    };
  } catch (error) {
    return {
      ok: false,
      message: error instanceof Error ? error.message : String(error)
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
    return missingEndpointInvokeResult(provider);
  }

  const payload = resolvePayload(input);
  const invokeId = randomUUID();
  const correlationId = `invoke-${invokeId}`;
  const headers = buildInvokeHeaders(
    provider,
    receipt.deploymentId,
    invokeId,
    correlationId,
    payload.contentType
  );
  const invokeResult = await invokeEndpoint(receipt.endpoint, payload, headers);
  if (invokeResult.ok) {
    await appendProviderLog(
      projectDir,
      provider,
      `invoke deploymentId=${receipt.deploymentId} invokeId=${invokeId} correlationId=${correlationId} endpoint=${receipt.endpoint} status=${invokeResult.statusCode}`
    );
    await persistInvokeCorrelation(projectDir, provider, receipt, invokeId);
    return {
      statusCode: invokeResult.statusCode,
      body: invokeResult.body,
      correlation: invokeCorrelation(receipt, invokeId)
    };
  }
  await appendProviderLog(
    projectDir,
    provider,
    `invoke failed deploymentId=${receipt.deploymentId} invokeId=${invokeId} correlationId=${correlationId} error=${invokeResult.message}`
  );
  await persistInvokeCorrelation(projectDir, provider, receipt, invokeId);
  return {
    statusCode: 502,
    body: JSON.stringify({
      error: "invoke failed",
      message: invokeResult.message
    }),
    correlation: invokeCorrelation(receipt, invokeId)
  };
}

export async function destroyProviderArtifacts(projectDir: string, provider: string): Promise<void> {
  const paths = providerDeployPaths(projectDir, provider);
  await rm(paths.deployDir, { recursive: true, force: true });
}

export {
  runJsonCommand,
  runShellCommand
} from "./provider-ops/command-runner";

export type { CommandResult } from "./provider-ops/command-runner";

export {
  parseProviderMetricsPayload,
  parseProviderTracesPayload,
  runProviderMetricsCommand,
  runProviderTracesCommand
} from "./provider-ops/payload";
