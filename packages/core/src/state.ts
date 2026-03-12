import type { ProjectStateConfig, StateBackendType } from "./project";
import { backendFromState, normalizeStateConfig } from "./state/config-utils";
import { LocalFileStateBackend } from "./state/file-backend";
import {
  AzBlobStateBackend,
  GcsStateBackend,
  PostgresStateBackend,
  S3StateBackend
} from "./state/keyvalue-backend";
import {
  computeStateEntriesChecksum,
  CURRENT_STATE_SCHEMA_VERSION,
  resolveAddress
} from "./state/record-utils";
import { readStateBackupFile, writeStateBackupFile } from "./state/backup";

export { normalizeStateConfig, CURRENT_STATE_SCHEMA_VERSION, computeStateEntriesChecksum, readStateBackupFile, writeStateBackupFile };
export { LocalFileStateBackend };

export type ProviderStateLifecycle = "in_progress" | "applied" | "failed";

export interface StateAddress {
  service: string;
  stage: string;
  provider: string;
}

export interface ProviderStateRecord {
  schemaVersion: number;
  provider: string;
  service: string;
  stage: string;
  updatedAt: string;
  lifecycle: ProviderStateLifecycle;
  endpoint?: string;
  resourceAddresses?: Record<string, string>;
  workflowAddresses?: Record<string, string>;
  secretReferences?: Record<string, string>;
  details?: Record<string, unknown>;
}

export interface StateLockInfo {
  backend: StateBackendType;
  lockId: string;
  owner: string;
  acquiredAt: string;
  heartbeatAt?: string;
  expiresAt: string;
}

export interface StateRecordEntry {
  address: StateAddress;
  record: ProviderStateRecord;
}

export interface StateLockEntry {
  address: StateAddress;
  lock: StateLockInfo;
}

export interface StateListFilter {
  service?: string;
  stage?: string;
  provider?: string;
}

export interface StateBackupSnapshot {
  schemaVersion: number;
  createdAt: string;
  backend: StateBackendType;
  records: StateRecordEntry[];
  locks: StateLockEntry[];
}

export interface ResolvedStateConfig {
  backend: StateBackendType;
  keyPrefix: string;
  lock: {
    enabled: boolean;
    timeoutSeconds: number;
    heartbeatSeconds: number;
    staleAfterSeconds: number;
  };
  local: {
    dir?: string;
  };
  postgres: {
    connectionStringEnv: string;
    schema: string;
    table: string;
  };
  s3: {
    bucket?: string;
    region?: string;
    keyPrefix: string;
    useLockfile: boolean;
  };
  gcs: {
    bucket?: string;
    prefix: string;
  };
  azblob: {
    container?: string;
    prefix: string;
  };
}

export interface StateBackend {
  readonly backend: StateBackendType;
  readonly config: ResolvedStateConfig;
  read(address: StateAddress): Promise<ProviderStateRecord | null>;
  write(address: StateAddress, state: ProviderStateRecord, lock?: StateLockInfo): Promise<void>;
  delete(address: StateAddress): Promise<void>;
  list(filter?: StateListFilter): Promise<StateRecordEntry[]>;
  lock(address: StateAddress, owner?: string): Promise<StateLockInfo>;
  renewLock(address: StateAddress, lock: StateLockInfo): Promise<StateLockInfo>;
  unlock(address: StateAddress, lock?: StateLockInfo): Promise<void>;
  forceUnlock(address: StateAddress): Promise<boolean>;
  readLock(address: StateAddress): Promise<StateLockInfo | null>;
  listLocks(filter?: StateListFilter): Promise<StateLockEntry[]>;
  createBackup(filter?: StateListFilter): Promise<StateBackupSnapshot>;
  restoreBackup(backup: StateBackupSnapshot): Promise<void>;
}

export interface LocalFileStateBackendOptions {
  projectDir: string;
  state?: ProjectStateConfig | ResolvedStateConfig;
}

export interface StateBackendFactoryOptions {
  projectDir: string;
  state?: ProjectStateConfig;
  backendOverride?: StateBackendType;
}

function createLocalBackend(
  projectDir: string,
  state: ProjectStateConfig | ResolvedStateConfig
): LocalFileStateBackend {
  return new LocalFileStateBackend({ projectDir, state });
}

export function createStateBackend(options: StateBackendFactoryOptions): StateBackend {
  const stateConfig = normalizeStateConfig(backendFromState(options.state, options.backendOverride));

  switch (stateConfig.backend) {
    case "local":
      return createLocalBackend(options.projectDir, stateConfig);
    case "postgres":
      return new PostgresStateBackend(stateConfig);
    case "s3":
      return new S3StateBackend(stateConfig);
    case "gcs":
      return new GcsStateBackend(stateConfig);
    case "azblob":
      return new AzBlobStateBackend(stateConfig);
    default:
      return createLocalBackend(options.projectDir, { ...stateConfig, backend: "local" });
  }
}

export async function migrateStateBetweenBackends(
  from: StateBackend,
  to: StateBackend,
  filter?: StateListFilter
): Promise<{
  copiedCount: number;
  fromChecksum: string;
  toChecksum: string;
}> {
  const fromEntries = await from.list(filter);
  for (const entry of fromEntries) {
    await to.write(entry.address, entry.record);
  }

  const toEntries = await to.list(filter);
  const fromChecksum = computeStateEntriesChecksum(fromEntries);
  const toChecksum = computeStateEntriesChecksum(toEntries);
  if (fromEntries.length !== toEntries.length || fromChecksum !== toChecksum) {
    throw new Error(
      `state migration verification failed: fromCount=${fromEntries.length} toCount=${toEntries.length} fromChecksum=${fromChecksum} toChecksum=${toChecksum}`
    );
  }

  return {
    copiedCount: fromEntries.length,
    fromChecksum,
    toChecksum
  };
}

export function stateAddressToKey(address: StateAddress): string {
  const safe = resolveAddress(address);
  return `${safe.service}/${safe.stage}/${safe.provider}`;
}
