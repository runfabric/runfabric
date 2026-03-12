import { isAbsolute, resolve } from "node:path";
import type { ProjectStateConfig, StateBackendType } from "../project";
import type { ResolvedStateConfig } from "../state";
import { isRecord } from "./record-utils";

function readNonEmptyString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value.trim() : undefined;
}

function readFiniteNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function normalizeLockConfig(state: ProjectStateConfig | undefined, timeoutSeconds: number) {
  const heartbeatSecondsValue = readFiniteNumber(state?.lock?.heartbeatSeconds);
  const staleAfterValue = readFiniteNumber(state?.lock?.staleAfterSeconds);
  const heartbeatSeconds =
    typeof heartbeatSecondsValue === "number"
      ? Math.max(1, heartbeatSecondsValue)
      : Math.max(1, Math.floor(timeoutSeconds / 3));
  const staleAfterSeconds =
    typeof staleAfterValue === "number" ? Math.max(timeoutSeconds, staleAfterValue) : timeoutSeconds * 2;

  return {
    enabled: state?.lock?.enabled !== false,
    timeoutSeconds,
    heartbeatSeconds,
    staleAfterSeconds
  };
}

function normalizePostgresConfig(state: ProjectStateConfig | undefined) {
  return {
    connectionStringEnv: readNonEmptyString(state?.postgres?.connectionStringEnv) || "RUNFABRIC_STATE_POSTGRES_URL",
    schema: readNonEmptyString(state?.postgres?.schema) || "public",
    table: readNonEmptyString(state?.postgres?.table) || "runfabric_state"
  };
}

function normalizeS3Config(state: ProjectStateConfig | undefined, keyPrefix: string) {
  return {
    bucket: readNonEmptyString(state?.s3?.bucket),
    region: readNonEmptyString(state?.s3?.region),
    keyPrefix: readNonEmptyString(state?.s3?.keyPrefix) || keyPrefix,
    useLockfile: state?.s3?.useLockfile !== false
  };
}

function normalizeGcsConfig(state: ProjectStateConfig | undefined, keyPrefix: string) {
  return {
    bucket: readNonEmptyString(state?.gcs?.bucket),
    prefix: readNonEmptyString(state?.gcs?.prefix) || keyPrefix
  };
}

function normalizeAzBlobConfig(state: ProjectStateConfig | undefined, keyPrefix: string) {
  return {
    container: readNonEmptyString(state?.azblob?.container),
    prefix: readNonEmptyString(state?.azblob?.prefix) || keyPrefix
  };
}

export function normalizeStateConfig(state?: ProjectStateConfig): ResolvedStateConfig {
  const keyPrefix = readNonEmptyString(state?.keyPrefix) || "runfabric/state";
  const timeoutSeconds = Math.max(1, readFiniteNumber(state?.lock?.timeoutSeconds) || 30);

  return {
    backend: state?.backend || "local",
    keyPrefix,
    lock: normalizeLockConfig(state, timeoutSeconds),
    local: {
      dir: readNonEmptyString(state?.local?.dir)
    },
    postgres: normalizePostgresConfig(state),
    s3: normalizeS3Config(state, keyPrefix),
    gcs: normalizeGcsConfig(state, keyPrefix),
    azblob: normalizeAzBlobConfig(state, keyPrefix)
  };
}

export function resolveBackendRootDir(projectDir: string, config: ResolvedStateConfig): string {
  if (config.local.dir) {
    return isAbsolute(config.local.dir) ? config.local.dir : resolve(projectDir, config.local.dir);
  }
  return resolve(projectDir, ".runfabric", "state");
}

export function isResolvedStateConfig(value: unknown): value is ResolvedStateConfig {
  return isRecord(value) && typeof value.backend === "string" && isRecord(value.lock);
}

export function backendFromState(
  state?: ProjectStateConfig,
  override?: StateBackendType
): ProjectStateConfig {
  if (!override) {
    return state || {};
  }
  return {
    ...(state || {}),
    backend: override
  };
}
