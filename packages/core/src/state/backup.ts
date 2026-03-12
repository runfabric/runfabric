import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import type { StateBackendType } from "../project";
import type { StateAddress, StateBackupSnapshot, StateLockEntry, StateLockInfo, StateRecordEntry } from "../state";
import {
  CURRENT_STATE_SCHEMA_VERSION,
  isRecord,
  migrateStateRecord,
  toIsoNow
} from "./record-utils";

const supportedBackends = ["local", "postgres", "s3", "gcs", "azblob"] as const;

function parseAddress(value: unknown): StateAddress {
  if (!isRecord(value)) {
    throw new Error("state backup record entry is invalid");
  }

  return {
    service: String(value.service || ""),
    stage: String(value.stage || ""),
    provider: String(value.provider || "")
  };
}

function parseRecordEntries(value: unknown): StateRecordEntry[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.map((entry) => {
    if (!isRecord(entry)) {
      throw new Error("state backup record entry is invalid");
    }
    return {
      address: parseAddress(entry.address),
      record: migrateStateRecord(entry.record)
    };
  });
}

function parseBackend(value: unknown): StateBackendType {
  if (typeof value !== "string" || !supportedBackends.includes(value as (typeof supportedBackends)[number])) {
    return "local";
  }
  return value as StateBackendType;
}

function parseLockInfo(value: unknown): StateLockInfo | null {
  if (!isRecord(value)) {
    return null;
  }

  return {
    backend: parseBackend(value.backend),
    lockId: String(value.lockId || ""),
    owner: String(value.owner || ""),
    acquiredAt: String(value.acquiredAt || toIsoNow()),
    heartbeatAt: typeof value.heartbeatAt === "string" ? value.heartbeatAt : undefined,
    expiresAt: String(value.expiresAt || toIsoNow())
  };
}

function parseLockEntries(value: unknown): StateLockEntry[] {
  if (!Array.isArray(value)) {
    return [];
  }

  const entries: StateLockEntry[] = [];
  for (const entry of value) {
    if (!isRecord(entry)) {
      continue;
    }
    const lock = parseLockInfo(entry.lock);
    if (!lock) {
      continue;
    }

    entries.push({
      address: parseAddress(entry.address),
      lock
    });
  }
  return entries;
}

export async function writeStateBackupFile(path: string, backup: StateBackupSnapshot): Promise<void> {
  await mkdir(dirname(path), { recursive: true });
  await writeFile(path, JSON.stringify(backup, null, 2), "utf8");
}

export async function readStateBackupFile(path: string): Promise<StateBackupSnapshot> {
  const content = await readFile(path, "utf8");
  const parsed = JSON.parse(content) as unknown;
  if (!isRecord(parsed)) {
    throw new Error("state backup file must be an object");
  }

  return {
    schemaVersion:
      typeof parsed.schemaVersion === "number" && Number.isFinite(parsed.schemaVersion)
        ? parsed.schemaVersion
        : CURRENT_STATE_SCHEMA_VERSION,
    createdAt: typeof parsed.createdAt === "string" ? parsed.createdAt : toIsoNow(),
    backend: parseBackend(parsed.backend),
    records: parseRecordEntries(parsed.records),
    locks: parseLockEntries(parsed.locks)
  };
}
