import type { ProjectConfig } from "@runfabric/core";
import {
  isRecord,
  readOptionalBooleanAtPath,
  readOptionalNumberAtPath
} from "./shared";

const supportedStateBackends = ["local", "postgres", "s3", "gcs", "azblob"] as const;
const supportedStateFields = ["backend", "keyPrefix", "lock", "local", "postgres", "s3", "gcs", "azblob"] as const;

type MutableStateConfig = NonNullable<ProjectConfig["state"]>;

type BackendBlockField = "bucket" | "region" | "keyPrefix" | "prefix" | "container";

function validateSupportedFields(value: Record<string, unknown>, path: string, errors: string[]): void {
  for (const key of Object.keys(value)) {
    if (!supportedStateFields.includes(key as (typeof supportedStateFields)[number])) {
      errors.push(`${path}.${key} is not a supported field`);
    }
  }
}

function readBackend(value: Record<string, unknown>, path: string, errors: string[]): MutableStateConfig["backend"] {
  if (!("backend" in value)) {
    return undefined;
  }

  const backend = value.backend;
  if (typeof backend !== "string" || !supportedStateBackends.includes(backend as (typeof supportedStateBackends)[number])) {
    errors.push(`${path}.backend must be one of: ${supportedStateBackends.join(", ")}`);
    return undefined;
  }

  return backend as MutableStateConfig["backend"];
}

function readKeyPrefix(value: Record<string, unknown>, path: string, errors: string[]): string | undefined {
  if (!("keyPrefix" in value)) {
    return undefined;
  }
  if (typeof value.keyPrefix !== "string" || value.keyPrefix.trim().length === 0) {
    errors.push(`${path}.keyPrefix must be a non-empty string`);
    return undefined;
  }
  return value.keyPrefix.trim();
}

function readLockConfig(value: unknown, path: string, errors: string[]): MutableStateConfig["lock"] {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path}.lock must be an object`);
    return undefined;
  }

  return {
    enabled: readOptionalBooleanAtPath(value.enabled, `${path}.lock.enabled`, errors),
    timeoutSeconds: readOptionalNumberAtPath(value.timeoutSeconds, `${path}.lock.timeoutSeconds`, errors, 1),
    heartbeatSeconds: readOptionalNumberAtPath(value.heartbeatSeconds, `${path}.lock.heartbeatSeconds`, errors, 1),
    staleAfterSeconds: readOptionalNumberAtPath(value.staleAfterSeconds, `${path}.lock.staleAfterSeconds`, errors, 1)
  };
}

function readLocalConfig(value: unknown, path: string, errors: string[]): MutableStateConfig["local"] {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path}.local must be an object`);
    return undefined;
  }

  if (!("dir" in value)) {
    return {};
  }
  if (typeof value.dir !== "string" || value.dir.trim().length === 0) {
    errors.push(`${path}.local.dir must be a non-empty string`);
    return {};
  }
  return { dir: value.dir.trim() };
}

function readPostgresConfig(value: unknown, path: string, errors: string[]): MutableStateConfig["postgres"] {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path}.postgres must be an object`);
    return undefined;
  }

  const out: MutableStateConfig["postgres"] = {};
  for (const field of ["connectionStringEnv", "schema", "table"] as const) {
    if (!(field in value)) {
      continue;
    }
    const fieldValue = value[field];
    if (typeof fieldValue !== "string" || fieldValue.trim().length === 0) {
      errors.push(`${path}.postgres.${field} must be a non-empty string`);
      continue;
    }
    out[field] = fieldValue.trim();
  }
  return out;
}

function readStringFieldBlock(
  value: unknown,
  path: string,
  block: "s3" | "gcs" | "azblob",
  fields: BackendBlockField[],
  errors: string[]
): Record<string, string> | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path}.${block} must be an object`);
    return undefined;
  }

  const out: Record<string, string> = {};
  for (const field of fields) {
    if (!(field in value)) {
      continue;
    }
    const fieldValue = value[field];
    if (typeof fieldValue !== "string" || fieldValue.trim().length === 0) {
      errors.push(`${path}.${block}.${field} must be a non-empty string`);
      continue;
    }
    out[field] = fieldValue.trim();
  }

  return out;
}

function readS3Config(value: unknown, path: string, errors: string[]): MutableStateConfig["s3"] {
  const block = readStringFieldBlock(value, path, "s3", ["bucket", "region", "keyPrefix"], errors);
  if (!block) {
    return undefined;
  }

  return {
    ...block,
    useLockfile: isRecord(value)
      ? readOptionalBooleanAtPath(value.useLockfile, `${path}.s3.useLockfile`, errors)
      : undefined
  };
}

function readGcsConfig(value: unknown, path: string, errors: string[]): MutableStateConfig["gcs"] {
  return readStringFieldBlock(value, path, "gcs", ["bucket", "prefix"], errors);
}

function readAzBlobConfig(value: unknown, path: string, errors: string[]): MutableStateConfig["azblob"] {
  return readStringFieldBlock(value, path, "azblob", ["container", "prefix"], errors);
}

function validateBackendSpecificRequirements(state: MutableStateConfig, path: string, errors: string[]): void {
  if (state.backend === "s3" && !state.s3?.bucket) {
    errors.push(`${path}.s3.bucket is required when state.backend is s3`);
  }
  if (state.backend === "gcs" && !state.gcs?.bucket) {
    errors.push(`${path}.gcs.bucket is required when state.backend is gcs`);
  }
  if (state.backend === "azblob" && !state.azblob?.container) {
    errors.push(`${path}.azblob.container is required when state.backend is azblob`);
  }
}

export function readStateConfigAtPath(
  value: unknown,
  path: string,
  errors: string[]
): ProjectConfig["state"] | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path} must be an object`);
    return undefined;
  }

  validateSupportedFields(value, path, errors);
  const state: MutableStateConfig = {
    backend: readBackend(value, path, errors),
    keyPrefix: readKeyPrefix(value, path, errors),
    lock: readLockConfig(value.lock, path, errors),
    local: readLocalConfig(value.local, path, errors),
    postgres: readPostgresConfig(value.postgres, path, errors),
    s3: readS3Config(value.s3, path, errors),
    gcs: readGcsConfig(value.gcs, path, errors),
    azblob: readAzBlobConfig(value.azblob, path, errors)
  };

  validateBackendSpecificRequirements(state, path, errors);
  return state;
}
