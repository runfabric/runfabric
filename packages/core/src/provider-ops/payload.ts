import type { MetricsResult, TraceRecord, TracesResult } from "../provider";
import { runJsonCommand } from "../provider-ops";

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
