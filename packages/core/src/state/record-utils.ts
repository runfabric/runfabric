import { createHash } from "node:crypto";
import type {
  ProviderStateLifecycle,
  ProviderStateRecord,
  StateAddress,
  StateRecordEntry
} from "../state";

export const STATE_FILE_SUFFIX = ".state.json";
export const LOCK_FILE_SUFFIX = ".state.json.lock";
export const CURRENT_STATE_SCHEMA_VERSION = 2;

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function normalizeAddressPart(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return "unknown";
  }
  return trimmed.replace(/[^a-zA-Z0-9._-]/g, "-");
}

export function resolveAddress(address: StateAddress): StateAddress {
  return {
    service: normalizeAddressPart(address.service),
    stage: normalizeAddressPart(address.stage),
    provider: normalizeAddressPart(address.provider)
  };
}

function sanitizeStateDetails(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((entry) => sanitizeStateDetails(entry));
  }

  if (isRecord(value)) {
    const out: Record<string, unknown> = {};
    for (const [key, entry] of Object.entries(value)) {
      if (/(secret|token|password|credential|private|api.?key|access.?key|session)/i.test(key)) {
        out[key] = "[REDACTED]";
      } else {
        out[key] = sanitizeStateDetails(entry);
      }
    }
    return out;
  }

  return value;
}

export function toIsoNow(): string {
  return new Date().toISOString();
}

function normalizeLifecycle(value: unknown): ProviderStateLifecycle {
  if (value === "in_progress" || value === "applied" || value === "failed") {
    return value;
  }
  return "applied";
}

export function sanitizeReferenceMap(value: unknown): Record<string, string> | undefined {
  if (!isRecord(value)) {
    return undefined;
  }

  const out: Record<string, string> = {};
  for (const [key, entryValue] of Object.entries(value)) {
    if (typeof entryValue !== "string" || entryValue.trim().length === 0) {
      continue;
    }
    out[key] = entryValue.trim();
  }
  return Object.keys(out).length > 0 ? out : undefined;
}

function readSchemaVersion(raw: Record<string, unknown>): number {
  const schemaVersionValue = raw.schemaVersion;
  const schemaVersion =
    typeof schemaVersionValue === "number" && Number.isFinite(schemaVersionValue)
      ? schemaVersionValue
      : 1;
  if (schemaVersion > CURRENT_STATE_SCHEMA_VERSION) {
    throw new Error(
      `state schema version ${schemaVersion} is newer than supported ${CURRENT_STATE_SCHEMA_VERSION}`
    );
  }
  return schemaVersion;
}

function readRequiredAddressFields(raw: Record<string, unknown>): Pick<ProviderStateRecord, "provider" | "service" | "stage"> {
  const provider = typeof raw.provider === "string" ? raw.provider : "";
  const service = typeof raw.service === "string" ? raw.service : "";
  const stage = typeof raw.stage === "string" ? raw.stage : "";
  if (!provider || !service || !stage) {
    throw new Error("state record is missing provider/service/stage");
  }
  return { provider, service, stage };
}

function readOptionalRecordFields(raw: Record<string, unknown>) {
  return {
    updatedAt: typeof raw.updatedAt === "string" ? raw.updatedAt : toIsoNow(),
    endpoint: typeof raw.endpoint === "string" ? raw.endpoint : undefined,
    resourceAddresses: sanitizeReferenceMap(raw.resourceAddresses),
    workflowAddresses: sanitizeReferenceMap(raw.workflowAddresses),
    secretReferences: sanitizeReferenceMap(raw.secretReferences),
    details: isRecord(raw.details)
      ? (sanitizeStateDetails(raw.details) as Record<string, unknown>)
      : undefined
  };
}

function migratedRecordForSchemaV1(
  required: Pick<ProviderStateRecord, "provider" | "service" | "stage">,
  optional: ReturnType<typeof readOptionalRecordFields>
): ProviderStateRecord {
  return {
    schemaVersion: CURRENT_STATE_SCHEMA_VERSION,
    provider: required.provider,
    service: required.service,
    stage: required.stage,
    updatedAt: optional.updatedAt,
    lifecycle: "applied",
    endpoint: optional.endpoint,
    resourceAddresses: optional.resourceAddresses,
    workflowAddresses: optional.workflowAddresses,
    secretReferences: optional.secretReferences,
    details: optional.details
  };
}

export function migrateStateRecord(raw: unknown): ProviderStateRecord {
  if (!isRecord(raw)) {
    throw new Error("state record must be an object");
  }

  const schemaVersion = readSchemaVersion(raw);
  const required = readRequiredAddressFields(raw);
  const optional = readOptionalRecordFields(raw);

  if (schemaVersion === 1) {
    return migratedRecordForSchemaV1(required, optional);
  }

  return {
    schemaVersion: CURRENT_STATE_SCHEMA_VERSION,
    provider: required.provider,
    service: required.service,
    stage: required.stage,
    updatedAt: optional.updatedAt,
    lifecycle: normalizeLifecycle(raw.lifecycle),
    endpoint: optional.endpoint,
    resourceAddresses: optional.resourceAddresses,
    workflowAddresses: optional.workflowAddresses,
    secretReferences: optional.secretReferences,
    details: optional.details
  };
}

export function normalizeRecord(record: ProviderStateRecord): ProviderStateRecord {
  return migrateStateRecord({
    ...record,
    schemaVersion: CURRENT_STATE_SCHEMA_VERSION,
    resourceAddresses: sanitizeReferenceMap(record.resourceAddresses || {}),
    workflowAddresses: sanitizeReferenceMap(record.workflowAddresses || {}),
    secretReferences: sanitizeReferenceMap(record.secretReferences || {}),
    details: sanitizeStateDetails(record.details || {})
  });
}

function checksumValue(input: string): string {
  return createHash("sha256").update(input).digest("hex");
}

function canonicalizeRecordEntry(entry: StateRecordEntry): string {
  return JSON.stringify({
    address: entry.address,
    record: entry.record
  });
}

export function computeStateEntriesChecksum(entries: StateRecordEntry[]): string {
  const canonical = [...entries]
    .sort((a, b) =>
      `${a.address.service}/${a.address.stage}/${a.address.provider}`.localeCompare(
        `${b.address.service}/${b.address.stage}/${b.address.provider}`
      )
    )
    .map((entry) => canonicalizeRecordEntry(entry))
    .join("\n");

  return checksumValue(canonical);
}
