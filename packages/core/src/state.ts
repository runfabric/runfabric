import { createHash, randomUUID } from "node:crypto";
import { mkdir, open, readdir, readFile, rm, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, isAbsolute, resolve } from "node:path";
import type { ProjectStateConfig, StateBackendType } from "./project";

const STATE_FILE_SUFFIX = ".state.json";
const LOCK_FILE_SUFFIX = ".state.json.lock";
const requireModule = createRequire(__filename);

export const CURRENT_STATE_SCHEMA_VERSION = 2;

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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function normalizeAddressPart(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return "unknown";
  }
  return trimmed.replace(/[^a-zA-Z0-9._-]/g, "-");
}

function toPathSegments(value: string): string[] {
  return value
    .split("/")
    .map((segment) => segment.trim())
    .filter(Boolean)
    .map((segment) => segment.replace(/[^a-zA-Z0-9._-]/g, "-"));
}

function resolveAddress(address: StateAddress): StateAddress {
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

function toIsoNow(): string {
  return new Date().toISOString();
}

function normalizeLifecycle(value: unknown): ProviderStateLifecycle {
  if (value === "in_progress" || value === "applied" || value === "failed") {
    return value;
  }
  return "applied";
}

function sanitizeReferenceMap(value: unknown): Record<string, string> | undefined {
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

function migrateStateRecord(raw: unknown): ProviderStateRecord {
  if (!isRecord(raw)) {
    throw new Error("state record must be an object");
  }

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

  const provider = typeof raw.provider === "string" ? raw.provider : "";
  const service = typeof raw.service === "string" ? raw.service : "";
  const stage = typeof raw.stage === "string" ? raw.stage : "";
  const updatedAt = typeof raw.updatedAt === "string" ? raw.updatedAt : toIsoNow();
  const endpoint = typeof raw.endpoint === "string" ? raw.endpoint : undefined;
  const resourceAddresses = sanitizeReferenceMap(raw.resourceAddresses);
  const workflowAddresses = sanitizeReferenceMap(raw.workflowAddresses);
  const secretReferences = sanitizeReferenceMap(raw.secretReferences);
  const details = isRecord(raw.details)
    ? (sanitizeStateDetails(raw.details) as Record<string, unknown>)
    : undefined;

  if (!provider || !service || !stage) {
    throw new Error("state record is missing provider/service/stage");
  }

  if (schemaVersion === 1) {
    return {
      schemaVersion: CURRENT_STATE_SCHEMA_VERSION,
      provider,
      service,
      stage,
      updatedAt,
      lifecycle: "applied",
      endpoint,
      resourceAddresses,
      workflowAddresses,
      secretReferences,
      details
    };
  }

  return {
    schemaVersion: CURRENT_STATE_SCHEMA_VERSION,
    provider,
    service,
    stage,
    updatedAt,
    lifecycle: normalizeLifecycle(raw.lifecycle),
    endpoint,
    resourceAddresses,
    workflowAddresses,
    secretReferences,
    details
  };
}

function normalizeRecord(record: ProviderStateRecord): ProviderStateRecord {
  return migrateStateRecord({
    ...record,
    schemaVersion: CURRENT_STATE_SCHEMA_VERSION,
    resourceAddresses: sanitizeReferenceMap(record.resourceAddresses || {}),
    workflowAddresses: sanitizeReferenceMap(record.workflowAddresses || {}),
    secretReferences: sanitizeReferenceMap(record.secretReferences || {}),
    details: sanitizeStateDetails(record.details || {})
  });
}

export function normalizeStateConfig(state?: ProjectStateConfig): ResolvedStateConfig {
  const keyPrefix =
    typeof state?.keyPrefix === "string" && state.keyPrefix.trim().length > 0
      ? state.keyPrefix.trim()
      : "runfabric/state";

  const timeoutSeconds =
    typeof state?.lock?.timeoutSeconds === "number" && Number.isFinite(state.lock.timeoutSeconds)
      ? Math.max(1, state.lock.timeoutSeconds)
      : 30;

  const heartbeatSeconds =
    typeof state?.lock?.heartbeatSeconds === "number" && Number.isFinite(state.lock.heartbeatSeconds)
      ? Math.max(1, state.lock.heartbeatSeconds)
      : Math.max(1, Math.floor(timeoutSeconds / 3));

  const staleAfterSeconds =
    typeof state?.lock?.staleAfterSeconds === "number" && Number.isFinite(state.lock.staleAfterSeconds)
      ? Math.max(timeoutSeconds, state.lock.staleAfterSeconds)
      : timeoutSeconds * 2;

  return {
    backend: state?.backend || "local",
    keyPrefix,
    lock: {
      enabled: state?.lock?.enabled !== false,
      timeoutSeconds,
      heartbeatSeconds,
      staleAfterSeconds
    },
    local: {
      dir:
        typeof state?.local?.dir === "string" && state.local.dir.trim().length > 0
          ? state.local.dir.trim()
          : undefined
    },
    postgres: {
      connectionStringEnv:
        typeof state?.postgres?.connectionStringEnv === "string" &&
        state.postgres.connectionStringEnv.trim().length > 0
          ? state.postgres.connectionStringEnv.trim()
          : "RUNFABRIC_STATE_POSTGRES_URL",
      schema:
        typeof state?.postgres?.schema === "string" && state.postgres.schema.trim().length > 0
          ? state.postgres.schema.trim()
          : "public",
      table:
        typeof state?.postgres?.table === "string" && state.postgres.table.trim().length > 0
          ? state.postgres.table.trim()
          : "runfabric_state"
    },
    s3: {
      bucket:
        typeof state?.s3?.bucket === "string" && state.s3.bucket.trim().length > 0
          ? state.s3.bucket.trim()
          : undefined,
      region:
        typeof state?.s3?.region === "string" && state.s3.region.trim().length > 0
          ? state.s3.region.trim()
          : undefined,
      keyPrefix:
        typeof state?.s3?.keyPrefix === "string" && state.s3.keyPrefix.trim().length > 0
          ? state.s3.keyPrefix.trim()
          : keyPrefix,
      useLockfile: state?.s3?.useLockfile !== false
    },
    gcs: {
      bucket:
        typeof state?.gcs?.bucket === "string" && state.gcs.bucket.trim().length > 0
          ? state.gcs.bucket.trim()
          : undefined,
      prefix:
        typeof state?.gcs?.prefix === "string" && state.gcs.prefix.trim().length > 0
          ? state.gcs.prefix.trim()
          : keyPrefix
    },
    azblob: {
      container:
        typeof state?.azblob?.container === "string" && state.azblob.container.trim().length > 0
          ? state.azblob.container.trim()
          : undefined,
      prefix:
        typeof state?.azblob?.prefix === "string" && state.azblob.prefix.trim().length > 0
          ? state.azblob.prefix.trim()
          : keyPrefix
    }
  };
}

function resolveBackendRootDir(projectDir: string, config: ResolvedStateConfig): string {
  if (config.local.dir) {
    return isAbsolute(config.local.dir) ? config.local.dir : resolve(projectDir, config.local.dir);
  }
  return resolve(projectDir, ".runfabric", "state");
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

class FileStateBackend implements StateBackend {
  readonly backend: StateBackendType;
  readonly config: ResolvedStateConfig;
  private readonly rootDir: string;

  constructor(options: { backend: StateBackendType; config: ResolvedStateConfig; rootDir: string }) {
    this.backend = options.backend;
    this.config = options.config;
    this.rootDir = options.rootDir;
  }

  private toStateFilePath(address: StateAddress): string {
    const safeAddress = resolveAddress(address);
    return resolve(this.rootDir, safeAddress.service, safeAddress.stage, `${safeAddress.provider}${STATE_FILE_SUFFIX}`);
  }

  private toLockFilePath(address: StateAddress): string {
    return `${this.toStateFilePath(address)}.lock`;
  }

  private async ensureParent(path: string): Promise<void> {
    await mkdir(dirname(path), { recursive: true });
  }

  private async listStateFiles(): Promise<string[]> {
    const out: string[] = [];
    try {
      const services = await readdir(this.rootDir, { withFileTypes: true });
      for (const service of services) {
        if (!service.isDirectory()) {
          continue;
        }
        const serviceDir = resolve(this.rootDir, service.name);
        const stages = await readdir(serviceDir, { withFileTypes: true });
        for (const stage of stages) {
          if (!stage.isDirectory()) {
            continue;
          }
          const stageDir = resolve(serviceDir, stage.name);
          const files = await readdir(stageDir, { withFileTypes: true });
          for (const file of files) {
            if (!file.isFile()) {
              continue;
            }
            if (file.name.endsWith(STATE_FILE_SUFFIX)) {
              out.push(resolve(stageDir, file.name));
            }
          }
        }
      }
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") {
        return [];
      }
      throw error;
    }
    return out;
  }

  private async listLockFiles(): Promise<string[]> {
    const out: string[] = [];
    try {
      const services = await readdir(this.rootDir, { withFileTypes: true });
      for (const service of services) {
        if (!service.isDirectory()) {
          continue;
        }
        const serviceDir = resolve(this.rootDir, service.name);
        const stages = await readdir(serviceDir, { withFileTypes: true });
        for (const stage of stages) {
          if (!stage.isDirectory()) {
            continue;
          }
          const stageDir = resolve(serviceDir, stage.name);
          const files = await readdir(stageDir, { withFileTypes: true });
          for (const file of files) {
            if (!file.isFile()) {
              continue;
            }
            if (file.name.endsWith(LOCK_FILE_SUFFIX)) {
              out.push(resolve(stageDir, file.name));
            }
          }
        }
      }
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") {
        return [];
      }
      throw error;
    }
    return out;
  }

  private extractAddressFromPath(path: string): StateAddress | null {
    const relative = path.slice(this.rootDir.length).replace(/^\/+/, "");
    const segments = relative.split("/");
    if (segments.length < 3) {
      return null;
    }
    const [service, stage, fileName] = segments;
    if (!service || !stage || !fileName) {
      return null;
    }
    const provider = fileName
      .replace(STATE_FILE_SUFFIX, "")
      .replace(LOCK_FILE_SUFFIX, "");
    if (!provider) {
      return null;
    }
    return { service, stage, provider };
  }

  private matchesFilter(
    address: StateAddress,
    record: ProviderStateRecord,
    filter?: StateListFilter
  ): boolean {
    if (!filter) {
      return true;
    }
    if (filter.service && filter.service !== record.service && filter.service !== address.service) {
      return false;
    }
    if (filter.stage && filter.stage !== record.stage && filter.stage !== address.stage) {
      return false;
    }
    if (filter.provider && filter.provider !== record.provider && filter.provider !== address.provider) {
      return false;
    }
    return true;
  }

  private lockError(address: StateAddress, existing: StateLockInfo | null): Error {
    const target = `${address.service}/${address.stage}/${address.provider}`;
    if (existing) {
      return new Error(
        `state lock is already held for ${target} by ${existing.owner} (lockId=${existing.lockId}, expiresAt=${existing.expiresAt}). Run "runfabric state force-unlock --service ${address.service} --stage ${address.stage} --provider ${address.provider}" if needed.`
      );
    }
    return new Error(
      `state lock is already held for ${target}. Run "runfabric state force-unlock --service ${address.service} --stage ${address.stage} --provider ${address.provider}" if needed.`
    );
  }

  private isStale(lock: StateLockInfo): boolean {
    const now = Date.now();
    const expiresAt = Date.parse(lock.expiresAt);
    if (Number.isFinite(expiresAt) && expiresAt <= now) {
      return true;
    }
    const heartbeatAt = Date.parse(lock.heartbeatAt || lock.acquiredAt);
    if (Number.isFinite(heartbeatAt)) {
      return heartbeatAt + this.config.lock.staleAfterSeconds * 1000 <= now;
    }
    return false;
  }

  async read(address: StateAddress): Promise<ProviderStateRecord | null> {
    const filePath = this.toStateFilePath(address);
    try {
      const content = await readFile(filePath, "utf8");
      return migrateStateRecord(JSON.parse(content) as unknown);
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") {
        return null;
      }
      throw error;
    }
  }

  async write(address: StateAddress, state: ProviderStateRecord, lock?: StateLockInfo): Promise<void> {
    const filePath = this.toStateFilePath(address);
    await this.ensureParent(filePath);

    if (this.config.lock.enabled) {
      const currentLock = await this.readLock(address);
      if (currentLock) {
        if (!lock || lock.lockId !== currentLock.lockId) {
          throw this.lockError(address, currentLock);
        }
      }
    }

    const normalized = normalizeRecord(state);
    await writeFile(filePath, JSON.stringify(normalized, null, 2), "utf8");
  }

  async delete(address: StateAddress): Promise<void> {
    const filePath = this.toStateFilePath(address);
    await rm(filePath, { force: true });
  }

  async list(filter?: StateListFilter): Promise<StateRecordEntry[]> {
    const files = await this.listStateFiles();
    const entries: StateRecordEntry[] = [];
    for (const filePath of files) {
      const address = this.extractAddressFromPath(filePath);
      if (!address) {
        continue;
      }
      try {
        const record = migrateStateRecord(JSON.parse(await readFile(filePath, "utf8")) as unknown);
        if (this.matchesFilter(address, record, filter)) {
          entries.push({ address, record });
        }
      } catch {
        continue;
      }
    }
    return entries;
  }

  async lock(address: StateAddress, owner = `pid:${process.pid}`): Promise<StateLockInfo> {
    if (!this.config.lock.enabled) {
      return {
        backend: this.backend,
        lockId: "locks-disabled",
        owner: "locks-disabled",
        acquiredAt: toIsoNow(),
        heartbeatAt: toIsoNow(),
        expiresAt: toIsoNow()
      };
    }

    const lockPath = this.toLockFilePath(address);
    await this.ensureParent(lockPath);

    for (let attempt = 0; attempt < 2; attempt += 1) {
      const acquiredAt = toIsoNow();
      const expiresAt = new Date(Date.now() + this.config.lock.timeoutSeconds * 1000).toISOString();
      const lockInfo: StateLockInfo = {
        backend: this.backend,
        lockId: randomUUID(),
        owner,
        acquiredAt,
        heartbeatAt: acquiredAt,
        expiresAt
      };

      try {
        const handle = await open(lockPath, "wx");
        await handle.writeFile(JSON.stringify(lockInfo, null, 2), "utf8");
        await handle.close();
        return lockInfo;
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code !== "EEXIST") {
          throw error;
        }

        const existing = await this.readLock(address);
        if (existing && this.isStale(existing)) {
          await rm(lockPath, { force: true });
          continue;
        }
        throw this.lockError(address, existing);
      }
    }

    throw this.lockError(address, await this.readLock(address));
  }

  async readLock(address: StateAddress): Promise<StateLockInfo | null> {
    const lockPath = this.toLockFilePath(address);
    try {
      const content = await readFile(lockPath, "utf8");
      const parsed = JSON.parse(content) as unknown;
      if (!isRecord(parsed)) {
        return null;
      }
      const lock: StateLockInfo = {
        backend: this.backend,
        lockId: typeof parsed.lockId === "string" ? parsed.lockId : "unknown",
        owner: typeof parsed.owner === "string" ? parsed.owner : "unknown",
        acquiredAt: typeof parsed.acquiredAt === "string" ? parsed.acquiredAt : toIsoNow(),
        heartbeatAt: typeof parsed.heartbeatAt === "string" ? parsed.heartbeatAt : undefined,
        expiresAt: typeof parsed.expiresAt === "string" ? parsed.expiresAt : toIsoNow()
      };
      return lock;
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") {
        return null;
      }
      throw error;
    }
  }

  async renewLock(address: StateAddress, lock: StateLockInfo): Promise<StateLockInfo> {
    if (!this.config.lock.enabled) {
      return lock;
    }

    const lockPath = this.toLockFilePath(address);
    const existing = await this.readLock(address);
    if (!existing) {
      throw new Error("cannot renew lock: no active lock found");
    }
    if (existing.lockId !== lock.lockId) {
      throw new Error(
        `cannot renew lock: lock token mismatch (expected ${existing.lockId}, received ${lock.lockId})`
      );
    }

    const now = toIsoNow();
    const refreshed: StateLockInfo = {
      ...existing,
      heartbeatAt: now,
      expiresAt: new Date(Date.now() + this.config.lock.timeoutSeconds * 1000).toISOString()
    };
    await writeFile(lockPath, JSON.stringify(refreshed, null, 2), "utf8");
    return refreshed;
  }

  async unlock(address: StateAddress, lock?: StateLockInfo): Promise<void> {
    if (!this.config.lock.enabled) {
      return;
    }

    const lockPath = this.toLockFilePath(address);
    if (lock) {
      const existing = await this.readLock(address);
      if (existing && existing.lockId !== lock.lockId) {
        throw new Error(
          `cannot unlock state: lock token mismatch (expected ${existing.lockId}, received ${lock.lockId})`
        );
      }
    }

    await rm(lockPath, { force: true });
  }

  async forceUnlock(address: StateAddress): Promise<boolean> {
    const existing = await this.readLock(address);
    await rm(this.toLockFilePath(address), { force: true });
    return !!existing;
  }

  async listLocks(filter?: StateListFilter): Promise<StateLockEntry[]> {
    const files = await this.listLockFiles();
    const entries: StateLockEntry[] = [];
    for (const filePath of files) {
      const address = this.extractAddressFromPath(filePath);
      if (!address) {
        continue;
      }
      const lock = await this.readLock(address);
      if (!lock) {
        continue;
      }

      if (filter?.service && filter.service !== address.service) {
        continue;
      }
      if (filter?.stage && filter.stage !== address.stage) {
        continue;
      }
      if (filter?.provider && filter.provider !== address.provider) {
        continue;
      }
      entries.push({ address, lock });
    }
    return entries;
  }

  async createBackup(filter?: StateListFilter): Promise<StateBackupSnapshot> {
    return {
      schemaVersion: CURRENT_STATE_SCHEMA_VERSION,
      createdAt: toIsoNow(),
      backend: this.backend,
      records: await this.list(filter),
      locks: await this.listLocks(filter)
    };
  }

  async restoreBackup(backup: StateBackupSnapshot): Promise<void> {
    if (backup.schemaVersion > CURRENT_STATE_SCHEMA_VERSION) {
      throw new Error(
        `backup schema version ${backup.schemaVersion} is newer than supported ${CURRENT_STATE_SCHEMA_VERSION}`
      );
    }

    for (const entry of backup.records) {
      await this.write(entry.address, entry.record);
    }

    if (!this.config.lock.enabled) {
      return;
    }

    for (const entry of backup.locks) {
      const lockPath = this.toLockFilePath(entry.address);
      await this.ensureParent(lockPath);
      await writeFile(lockPath, JSON.stringify(entry.lock, null, 2), "utf8");
    }
  }
}

function isResolvedStateConfig(value: unknown): value is ResolvedStateConfig {
  return isRecord(value) && typeof value.backend === "string" && isRecord(value.lock);
}

function backendFromState(state?: ProjectStateConfig, override?: StateBackendType): ProjectStateConfig {
  if (!override) {
    return state || {};
  }
  return {
    ...(state || {}),
    backend: override
  };
}

export class LocalFileStateBackend extends FileStateBackend {
  constructor(options: LocalFileStateBackendOptions) {
    const config = isResolvedStateConfig(options.state)
      ? options.state
      : normalizeStateConfig(options.state);
    super({
      backend: "local",
      config,
      rootDir: resolveBackendRootDir(options.projectDir, { ...config, backend: "local" })
    });
  }
}

interface KeyValueStore {
  get(key: string): Promise<string | null>;
  put(key: string, value: string): Promise<void>;
  putIfAbsent?(key: string, value: string): Promise<boolean>;
  delete(key: string): Promise<void>;
  list(prefix: string): Promise<string[]>;
}

function normalizeObjectPrefix(value: string): string {
  return value
    .split("/")
    .map((segment) => segment.trim())
    .filter(Boolean)
    .join("/");
}

function requireOptionalModule<T>(moduleName: string, installHint: string): T {
  try {
    return requireModule(moduleName) as T;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "MODULE_NOT_FOUND") {
      throw new Error(`missing optional dependency "${moduleName}". ${installHint}`);
    }
    throw error;
  }
}

function isErrnoWithCode(error: unknown, code: string): boolean {
  return isRecord(error) && typeof error.code === "string" && error.code === code;
}

function isHttpStatusError(error: unknown, statusCode: number): boolean {
  return (
    isRecord(error) &&
    typeof error.statusCode === "number" &&
    Number.isFinite(error.statusCode) &&
    error.statusCode === statusCode
  );
}

function quotePostgresIdentifier(value: string, field: string): string {
  if (!/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(value)) {
    throw new Error(
      `invalid postgres identifier for ${field}: "${value}". Use letters, numbers, underscore and start with a letter or underscore.`
    );
  }
  return `"${value}"`;
}

function normalizeBackendKeyPrefix(config: ResolvedStateConfig): string {
  switch (config.backend) {
    case "s3":
      return normalizeObjectPrefix(config.s3.keyPrefix);
    case "gcs":
      return normalizeObjectPrefix(config.gcs.prefix);
    case "azblob":
      return normalizeObjectPrefix(config.azblob.prefix);
    default:
      return normalizeObjectPrefix(config.keyPrefix);
  }
}

function formatBackendCredentialError(backend: StateBackendType, detail: string): Error {
  return new Error(`state backend "${backend}" credential/config error: ${detail}`);
}

async function streamToUtf8(input: unknown): Promise<string> {
  if (!input) {
    return "";
  }
  if (typeof input === "string") {
    return input;
  }
  if (Buffer.isBuffer(input)) {
    return input.toString("utf8");
  }

  if (isRecord(input) && typeof input.transformToString === "function") {
    const value = await input.transformToString("utf8");
    return typeof value === "string" ? value : String(value ?? "");
  }

  if (isRecord(input) && typeof input.getReader === "function") {
    const reader = input.getReader();
    const chunks: Uint8Array[] = [];
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      chunks.push(value);
    }
    return Buffer.concat(chunks.map((chunk) => Buffer.from(chunk))).toString("utf8");
  }

  const maybeAsyncIterable = input as { [Symbol.asyncIterator]?: () => AsyncIterator<unknown> };
  if (
    typeof input === "object" &&
    input !== null &&
    typeof maybeAsyncIterable[Symbol.asyncIterator] === "function"
  ) {
    const chunks: Buffer[] = [];
    for await (const chunk of maybeAsyncIterable as unknown as AsyncIterable<unknown>) {
      if (typeof chunk === "string") {
        chunks.push(Buffer.from(chunk));
      } else if (chunk instanceof Uint8Array) {
        chunks.push(Buffer.from(chunk));
      } else if (Buffer.isBuffer(chunk)) {
        chunks.push(chunk);
      }
    }
    return Buffer.concat(chunks).toString("utf8");
  }

  return String(input);
}

class KeyValueStateBackend implements StateBackend {
  readonly backend: StateBackendType;
  readonly config: ResolvedStateConfig;
  private readonly keyPrefix: string;
  private readonly store: KeyValueStore;

  constructor(options: {
    backend: StateBackendType;
    config: ResolvedStateConfig;
    keyPrefix: string;
    store: KeyValueStore;
  }) {
    this.backend = options.backend;
    this.config = options.config;
    this.keyPrefix = normalizeObjectPrefix(options.keyPrefix);
    this.store = options.store;
  }

  private objectPrefix(): string {
    return this.keyPrefix.length > 0 ? `${this.keyPrefix}/` : "";
  }

  private toStateKey(address: StateAddress): string {
    const safeAddress = resolveAddress(address);
    return `${this.objectPrefix()}${safeAddress.service}/${safeAddress.stage}/${safeAddress.provider}${STATE_FILE_SUFFIX}`;
  }

  private toLockKey(address: StateAddress): string {
    return `${this.toStateKey(address)}.lock`;
  }

  private extractAddressFromKey(key: string): StateAddress | null {
    const prefix = this.objectPrefix();
    if (prefix.length > 0 && !key.startsWith(prefix)) {
      return null;
    }
    const relative = prefix.length > 0 ? key.slice(prefix.length) : key;
    const segments = relative.split("/");
    if (segments.length < 3) {
      return null;
    }
    const [service, stage, fileName] = segments;
    if (!service || !stage || !fileName) {
      return null;
    }
    const provider = fileName.replace(STATE_FILE_SUFFIX, "").replace(LOCK_FILE_SUFFIX, "");
    if (!provider) {
      return null;
    }
    return {
      service,
      stage,
      provider
    };
  }

  private matchesFilter(
    address: StateAddress,
    record: ProviderStateRecord,
    filter?: StateListFilter
  ): boolean {
    if (!filter) {
      return true;
    }
    if (filter.service && filter.service !== record.service && filter.service !== address.service) {
      return false;
    }
    if (filter.stage && filter.stage !== record.stage && filter.stage !== address.stage) {
      return false;
    }
    if (filter.provider && filter.provider !== record.provider && filter.provider !== address.provider) {
      return false;
    }
    return true;
  }

  private lockError(address: StateAddress, existing: StateLockInfo | null): Error {
    const target = `${address.service}/${address.stage}/${address.provider}`;
    if (existing) {
      return new Error(
        `state lock is already held for ${target} by ${existing.owner} (lockId=${existing.lockId}, expiresAt=${existing.expiresAt}). Run "runfabric state force-unlock --service ${address.service} --stage ${address.stage} --provider ${address.provider}" if needed.`
      );
    }
    return new Error(
      `state lock is already held for ${target}. Run "runfabric state force-unlock --service ${address.service} --stage ${address.stage} --provider ${address.provider}" if needed.`
    );
  }

  private isStale(lock: StateLockInfo): boolean {
    const now = Date.now();
    const expiresAt = Date.parse(lock.expiresAt);
    if (Number.isFinite(expiresAt) && expiresAt <= now) {
      return true;
    }
    const heartbeatAt = Date.parse(lock.heartbeatAt || lock.acquiredAt);
    if (Number.isFinite(heartbeatAt)) {
      return heartbeatAt + this.config.lock.staleAfterSeconds * 1000 <= now;
    }
    return false;
  }

  async read(address: StateAddress): Promise<ProviderStateRecord | null> {
    const content = await this.store.get(this.toStateKey(address));
    if (!content) {
      return null;
    }
    return migrateStateRecord(JSON.parse(content) as unknown);
  }

  async write(address: StateAddress, state: ProviderStateRecord, lock?: StateLockInfo): Promise<void> {
    if (this.config.lock.enabled) {
      const currentLock = await this.readLock(address);
      if (currentLock) {
        if (!lock || lock.lockId !== currentLock.lockId) {
          throw this.lockError(address, currentLock);
        }
      }
    }

    const normalized = normalizeRecord(state);
    await this.store.put(this.toStateKey(address), JSON.stringify(normalized, null, 2));
  }

  async delete(address: StateAddress): Promise<void> {
    await this.store.delete(this.toStateKey(address));
  }

  async list(filter?: StateListFilter): Promise<StateRecordEntry[]> {
    const keys = await this.store.list(this.objectPrefix());
    const entries: StateRecordEntry[] = [];
    for (const key of keys) {
      if (!key.endsWith(STATE_FILE_SUFFIX) || key.endsWith(LOCK_FILE_SUFFIX)) {
        continue;
      }
      const address = this.extractAddressFromKey(key);
      if (!address) {
        continue;
      }

      try {
        const content = await this.store.get(key);
        if (!content) {
          continue;
        }
        const record = migrateStateRecord(JSON.parse(content) as unknown);
        if (this.matchesFilter(address, record, filter)) {
          entries.push({ address, record });
        }
      } catch {
        continue;
      }
    }

    return entries;
  }

  async lock(address: StateAddress, owner = `pid:${process.pid}`): Promise<StateLockInfo> {
    if (!this.config.lock.enabled) {
      return {
        backend: this.backend,
        lockId: "locks-disabled",
        owner: "locks-disabled",
        acquiredAt: toIsoNow(),
        heartbeatAt: toIsoNow(),
        expiresAt: toIsoNow()
      };
    }

    const lockKey = this.toLockKey(address);
    for (let attempt = 0; attempt < 2; attempt += 1) {
      const acquiredAt = toIsoNow();
      const lockInfo: StateLockInfo = {
        backend: this.backend,
        lockId: randomUUID(),
        owner,
        acquiredAt,
        heartbeatAt: acquiredAt,
        expiresAt: new Date(Date.now() + this.config.lock.timeoutSeconds * 1000).toISOString()
      };
      const serialized = JSON.stringify(lockInfo, null, 2);

      if (this.store.putIfAbsent) {
        const inserted = await this.store.putIfAbsent(lockKey, serialized);
        if (inserted) {
          return lockInfo;
        }
      } else {
        const existing = await this.readLock(address);
        if (!existing) {
          await this.store.put(lockKey, serialized);
          const verify = await this.readLock(address);
          if (verify && verify.lockId === lockInfo.lockId) {
            return lockInfo;
          }
        }
      }

      const existing = await this.readLock(address);
      if (existing && this.isStale(existing)) {
        await this.store.delete(lockKey);
        continue;
      }

      throw this.lockError(address, existing);
    }

    throw this.lockError(address, await this.readLock(address));
  }

  async readLock(address: StateAddress): Promise<StateLockInfo | null> {
    const content = await this.store.get(this.toLockKey(address));
    if (!content) {
      return null;
    }
    const parsed = JSON.parse(content) as unknown;
    if (!isRecord(parsed)) {
      return null;
    }
    return {
      backend: this.backend,
      lockId: typeof parsed.lockId === "string" ? parsed.lockId : "unknown",
      owner: typeof parsed.owner === "string" ? parsed.owner : "unknown",
      acquiredAt: typeof parsed.acquiredAt === "string" ? parsed.acquiredAt : toIsoNow(),
      heartbeatAt: typeof parsed.heartbeatAt === "string" ? parsed.heartbeatAt : undefined,
      expiresAt: typeof parsed.expiresAt === "string" ? parsed.expiresAt : toIsoNow()
    };
  }

  async renewLock(address: StateAddress, lock: StateLockInfo): Promise<StateLockInfo> {
    if (!this.config.lock.enabled) {
      return lock;
    }

    const existing = await this.readLock(address);
    if (!existing) {
      throw new Error("cannot renew lock: no active lock found");
    }
    if (existing.lockId !== lock.lockId) {
      throw new Error(
        `cannot renew lock: lock token mismatch (expected ${existing.lockId}, received ${lock.lockId})`
      );
    }

    const now = toIsoNow();
    const refreshed: StateLockInfo = {
      ...existing,
      heartbeatAt: now,
      expiresAt: new Date(Date.now() + this.config.lock.timeoutSeconds * 1000).toISOString()
    };
    await this.store.put(this.toLockKey(address), JSON.stringify(refreshed, null, 2));
    return refreshed;
  }

  async unlock(address: StateAddress, lock?: StateLockInfo): Promise<void> {
    if (!this.config.lock.enabled) {
      return;
    }
    if (lock) {
      const existing = await this.readLock(address);
      if (existing && existing.lockId !== lock.lockId) {
        throw new Error(
          `cannot unlock state: lock token mismatch (expected ${existing.lockId}, received ${lock.lockId})`
        );
      }
    }
    await this.store.delete(this.toLockKey(address));
  }

  async forceUnlock(address: StateAddress): Promise<boolean> {
    const existing = await this.readLock(address);
    await this.store.delete(this.toLockKey(address));
    return !!existing;
  }

  async listLocks(filter?: StateListFilter): Promise<StateLockEntry[]> {
    const keys = await this.store.list(this.objectPrefix());
    const entries: StateLockEntry[] = [];
    for (const key of keys) {
      if (!key.endsWith(LOCK_FILE_SUFFIX)) {
        continue;
      }
      const address = this.extractAddressFromKey(key);
      if (!address) {
        continue;
      }
      if (filter?.service && filter.service !== address.service) {
        continue;
      }
      if (filter?.stage && filter.stage !== address.stage) {
        continue;
      }
      if (filter?.provider && filter.provider !== address.provider) {
        continue;
      }
      const lock = await this.readLock(address);
      if (!lock) {
        continue;
      }
      entries.push({ address, lock });
    }
    return entries;
  }

  async createBackup(filter?: StateListFilter): Promise<StateBackupSnapshot> {
    return {
      schemaVersion: CURRENT_STATE_SCHEMA_VERSION,
      createdAt: toIsoNow(),
      backend: this.backend,
      records: await this.list(filter),
      locks: await this.listLocks(filter)
    };
  }

  async restoreBackup(backup: StateBackupSnapshot): Promise<void> {
    if (backup.schemaVersion > CURRENT_STATE_SCHEMA_VERSION) {
      throw new Error(
        `backup schema version ${backup.schemaVersion} is newer than supported ${CURRENT_STATE_SCHEMA_VERSION}`
      );
    }

    for (const entry of backup.records) {
      await this.write(entry.address, entry.record);
    }

    if (!this.config.lock.enabled) {
      return;
    }

    for (const entry of backup.locks) {
      await this.store.put(this.toLockKey(entry.address), JSON.stringify(entry.lock, null, 2));
    }
  }
}

class PostgresKeyValueStore implements KeyValueStore {
  private readonly connectionStringEnv: string;
  private readonly schema: string;
  private readonly table: string;
  private readonly tableRef: string;
  private pool:
    | {
        query: (
          queryText: string,
          values?: unknown[]
        ) => Promise<{ rows: Array<Record<string, unknown>>; rowCount: number | null }>;
      }
    | undefined;
  private initPromise: Promise<void> | undefined;

  constructor(config: ResolvedStateConfig) {
    this.connectionStringEnv = config.postgres.connectionStringEnv;
    this.schema = config.postgres.schema;
    this.table = config.postgres.table;
    const schemaSql = quotePostgresIdentifier(this.schema, "state.postgres.schema");
    const tableSql = quotePostgresIdentifier(this.table, "state.postgres.table");
    this.tableRef = `${schemaSql}.${tableSql}`;
  }

  private async init(): Promise<void> {
    if (this.initPromise) {
      return this.initPromise;
    }

    this.initPromise = (async () => {
      const connectionString = process.env[this.connectionStringEnv];
      if (!connectionString || connectionString.trim().length === 0) {
        throw formatBackendCredentialError(
          "postgres",
          `set ${this.connectionStringEnv} with a valid postgres connection string`
        );
      }

      const pg = requireOptionalModule<{ Pool: new (options: { connectionString: string }) => unknown }>(
        "pg",
        "Install with: pnpm add -w --filter @runfabric/core pg"
      );
      const poolCandidate = new pg.Pool({ connectionString });
      if (!isRecord(poolCandidate) || typeof poolCandidate.query !== "function") {
        throw new Error("pg Pool initialization failed");
      }
      this.pool = poolCandidate as {
        query: (
          queryText: string,
          values?: unknown[]
        ) => Promise<{ rows: Array<Record<string, unknown>>; rowCount: number | null }>;
      };

      await this.pool.query(
        `CREATE SCHEMA IF NOT EXISTS ${quotePostgresIdentifier(this.schema, "state.postgres.schema")}`
      );
      await this.pool.query(
        `CREATE TABLE IF NOT EXISTS ${this.tableRef} (
          key TEXT PRIMARY KEY,
          value TEXT NOT NULL,
          updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`
      );
    })();

    return this.initPromise;
  }

  private async query(
    queryText: string,
    values?: unknown[]
  ): Promise<{ rows: Array<Record<string, unknown>>; rowCount: number | null }> {
    await this.init();
    if (!this.pool) {
      throw new Error("postgres pool was not initialized");
    }
    return this.pool.query(queryText, values);
  }

  async get(key: string): Promise<string | null> {
    const result = await this.query(`SELECT value FROM ${this.tableRef} WHERE key = $1`, [key]);
    if (result.rows.length === 0) {
      return null;
    }
    const value = result.rows[0].value;
    return typeof value === "string" ? value : null;
  }

  async put(key: string, value: string): Promise<void> {
    await this.query(
      `INSERT INTO ${this.tableRef} (key, value, updated_at)
       VALUES ($1, $2, now())
       ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
      [key, value]
    );
  }

  async putIfAbsent(key: string, value: string): Promise<boolean> {
    const result = await this.query(
      `INSERT INTO ${this.tableRef} (key, value, updated_at)
       VALUES ($1, $2, now())
       ON CONFLICT (key) DO NOTHING`,
      [key, value]
    );
    return (result.rowCount || 0) > 0;
  }

  async delete(key: string): Promise<void> {
    await this.query(`DELETE FROM ${this.tableRef} WHERE key = $1`, [key]);
  }

  async list(prefix: string): Promise<string[]> {
    const likePrefix = `${prefix}%`;
    const result = await this.query(`SELECT key FROM ${this.tableRef} WHERE key LIKE $1 ORDER BY key ASC`, [
      likePrefix
    ]);
    return result.rows
      .map((row) => row.key)
      .filter((value): value is string => typeof value === "string");
  }
}

class S3KeyValueStore implements KeyValueStore {
  private readonly bucket: string;
  private readonly client: { send: (command: unknown) => Promise<Record<string, unknown>> };
  private readonly commands: {
    GetObjectCommand: new (input: Record<string, unknown>) => unknown;
    PutObjectCommand: new (input: Record<string, unknown>) => unknown;
    DeleteObjectCommand: new (input: Record<string, unknown>) => unknown;
    ListObjectsV2Command: new (input: Record<string, unknown>) => unknown;
  };

  constructor(config: ResolvedStateConfig) {
    if (!config.s3.bucket) {
      throw formatBackendCredentialError("s3", "set state.s3.bucket in runfabric.yml");
    }
    this.bucket = config.s3.bucket;

    const aws = requireOptionalModule<{
      S3Client: new (options: Record<string, unknown>) => { send: (command: unknown) => Promise<Record<string, unknown>> };
      GetObjectCommand: new (input: Record<string, unknown>) => unknown;
      PutObjectCommand: new (input: Record<string, unknown>) => unknown;
      DeleteObjectCommand: new (input: Record<string, unknown>) => unknown;
      ListObjectsV2Command: new (input: Record<string, unknown>) => unknown;
    }>("@aws-sdk/client-s3", "Install with: pnpm add -w --filter @runfabric/core @aws-sdk/client-s3");

    this.client = new aws.S3Client({
      ...(config.s3.region ? { region: config.s3.region } : {})
    });
    this.commands = {
      GetObjectCommand: aws.GetObjectCommand,
      PutObjectCommand: aws.PutObjectCommand,
      DeleteObjectCommand: aws.DeleteObjectCommand,
      ListObjectsV2Command: aws.ListObjectsV2Command
    };
  }

  async get(key: string): Promise<string | null> {
    try {
      const response = await this.client.send(
        new this.commands.GetObjectCommand({
          Bucket: this.bucket,
          Key: key
        })
      );
      return streamToUtf8(response.Body);
    } catch (error) {
      if (
        isErrnoWithCode(error, "NoSuchKey") ||
        isErrnoWithCode(error, "NotFound") ||
        isHttpStatusError(error, 404)
      ) {
        return null;
      }
      throw error;
    }
  }

  async put(key: string, value: string): Promise<void> {
    await this.client.send(
      new this.commands.PutObjectCommand({
        Bucket: this.bucket,
        Key: key,
        Body: value,
        ContentType: "application/json; charset=utf-8"
      })
    );
  }

  async putIfAbsent(key: string, value: string): Promise<boolean> {
    try {
      await this.client.send(
        new this.commands.PutObjectCommand({
          Bucket: this.bucket,
          Key: key,
          Body: value,
          ContentType: "application/json; charset=utf-8",
          IfNoneMatch: "*"
        })
      );
      return true;
    } catch (error) {
      if (isErrnoWithCode(error, "PreconditionFailed") || isHttpStatusError(error, 412)) {
        return false;
      }
      throw error;
    }
  }

  async delete(key: string): Promise<void> {
    await this.client.send(
      new this.commands.DeleteObjectCommand({
        Bucket: this.bucket,
        Key: key
      })
    );
  }

  async list(prefix: string): Promise<string[]> {
    const out: string[] = [];
    let continuationToken: string | undefined;
    do {
      const response = await this.client.send(
        new this.commands.ListObjectsV2Command({
          Bucket: this.bucket,
          Prefix: prefix,
          ContinuationToken: continuationToken
        })
      );
      const contents = Array.isArray(response.Contents) ? response.Contents : [];
      for (const entry of contents) {
        if (isRecord(entry) && typeof entry.Key === "string") {
          out.push(entry.Key);
        }
      }
      continuationToken =
        typeof response.NextContinuationToken === "string" ? response.NextContinuationToken : undefined;
    } while (continuationToken);
    return out;
  }
}

class GcsKeyValueStore implements KeyValueStore {
  private readonly bucket: {
    file: (name: string) => {
      download: () => Promise<[Buffer]>;
      save: (data: string, options?: Record<string, unknown>) => Promise<void>;
      delete: (options?: Record<string, unknown>) => Promise<void>;
    };
    getFiles: (options?: Record<string, unknown>) => Promise<[Array<{ name: string }>, Record<string, unknown>?]>;
  };

  constructor(config: ResolvedStateConfig) {
    if (!config.gcs.bucket) {
      throw formatBackendCredentialError("gcs", "set state.gcs.bucket in runfabric.yml");
    }
    const gcs = requireOptionalModule<{ Storage: new () => { bucket: (name: string) => unknown } }>(
      "@google-cloud/storage",
      "Install with: pnpm add -w --filter @runfabric/core @google-cloud/storage"
    );
    const storage = new gcs.Storage();
    const bucketCandidate = storage.bucket(config.gcs.bucket);
    if (!isRecord(bucketCandidate) || typeof bucketCandidate.file !== "function") {
      throw new Error("gcs bucket initialization failed");
    }
    this.bucket = bucketCandidate as {
      file: (name: string) => {
        download: () => Promise<[Buffer]>;
        save: (data: string, options?: Record<string, unknown>) => Promise<void>;
        delete: (options?: Record<string, unknown>) => Promise<void>;
      };
      getFiles: (
        options?: Record<string, unknown>
      ) => Promise<[Array<{ name: string }>, Record<string, unknown>?]>;
    };
  }

  async get(key: string): Promise<string | null> {
    const file = this.bucket.file(key);
    try {
      const [content] = await file.download();
      return content.toString("utf8");
    } catch (error) {
      if (isErrnoWithCode(error, "404") || isHttpStatusError(error, 404)) {
        return null;
      }
      throw error;
    }
  }

  async put(key: string, value: string): Promise<void> {
    const file = this.bucket.file(key);
    await file.save(value, {
      resumable: false,
      contentType: "application/json; charset=utf-8"
    });
  }

  async putIfAbsent(key: string, value: string): Promise<boolean> {
    const file = this.bucket.file(key);
    try {
      await file.save(value, {
        resumable: false,
        contentType: "application/json; charset=utf-8",
        preconditionOpts: {
          ifGenerationMatch: 0
        }
      });
      return true;
    } catch (error) {
      if (isErrnoWithCode(error, "412") || isHttpStatusError(error, 412)) {
        return false;
      }
      throw error;
    }
  }

  async delete(key: string): Promise<void> {
    await this.bucket.file(key).delete({
      ignoreNotFound: true
    });
  }

  async list(prefix: string): Promise<string[]> {
    const keys: string[] = [];
    let pageToken: string | undefined;
    do {
      const [files, nextQuery] = await this.bucket.getFiles({
        prefix,
        pageToken,
        autoPaginate: false
      });
      for (const file of files) {
        if (file && typeof file.name === "string") {
          keys.push(file.name);
        }
      }
      pageToken =
        nextQuery && typeof nextQuery.pageToken === "string" ? nextQuery.pageToken : undefined;
    } while (pageToken);

    return keys;
  }
}

class AzBlobKeyValueStore implements KeyValueStore {
  private readonly container: {
    createIfNotExists: () => Promise<unknown>;
    getBlockBlobClient: (name: string) => {
      upload: (data: string, length: number, options?: Record<string, unknown>) => Promise<unknown>;
      deleteIfExists: () => Promise<unknown>;
      download: () => Promise<{ readableStreamBody?: unknown }>;
    };
    listBlobsFlat: (options?: Record<string, unknown>) => AsyncIterable<{ name?: string }>;
  };
  private ensureContainerPromise: Promise<void> | undefined;

  constructor(config: ResolvedStateConfig) {
    if (!config.azblob.container) {
      throw formatBackendCredentialError("azblob", "set state.azblob.container in runfabric.yml");
    }

    const az = requireOptionalModule<{
      BlobServiceClient: {
        fromConnectionString: (value: string) => {
          getContainerClient: (containerName: string) => unknown;
        };
        new (url: string, credential: unknown): {
          getContainerClient: (containerName: string) => unknown;
        };
      };
      StorageSharedKeyCredential: new (accountName: string, accountKey: string) => unknown;
    }>(
      "@azure/storage-blob",
      "Install with: pnpm add -w --filter @runfabric/core @azure/storage-blob"
    );

    let serviceClient:
      | {
          getContainerClient: (containerName: string) => unknown;
        }
      | undefined;

    const connectionString = process.env.AZURE_STORAGE_CONNECTION_STRING;
    if (connectionString && connectionString.trim().length > 0) {
      serviceClient = az.BlobServiceClient.fromConnectionString(connectionString.trim());
    } else {
      const account = process.env.AZURE_STORAGE_ACCOUNT;
      const key = process.env.AZURE_STORAGE_KEY;
      if (!account || !key) {
        throw formatBackendCredentialError(
          "azblob",
          "set AZURE_STORAGE_CONNECTION_STRING or AZURE_STORAGE_ACCOUNT + AZURE_STORAGE_KEY"
        );
      }
      const credential = new az.StorageSharedKeyCredential(account, key);
      const candidate = new az.BlobServiceClient(`https://${account}.blob.core.windows.net`, credential);
      if (!isRecord(candidate) || typeof candidate.getContainerClient !== "function") {
        throw new Error("azblob service client initialization failed");
      }
      serviceClient = candidate as { getContainerClient: (containerName: string) => unknown };
    }

    const containerCandidate = serviceClient.getContainerClient(config.azblob.container);
    if (!isRecord(containerCandidate) || typeof containerCandidate.getBlockBlobClient !== "function") {
      throw new Error("azblob container client initialization failed");
    }
    this.container = containerCandidate as {
      createIfNotExists: () => Promise<unknown>;
      getBlockBlobClient: (name: string) => {
        upload: (data: string, length: number, options?: Record<string, unknown>) => Promise<unknown>;
        deleteIfExists: () => Promise<unknown>;
        download: () => Promise<{ readableStreamBody?: unknown }>;
      };
      listBlobsFlat: (options?: Record<string, unknown>) => AsyncIterable<{ name?: string }>;
    };
  }

  private async ensureContainer(): Promise<void> {
    if (!this.ensureContainerPromise) {
      this.ensureContainerPromise = (async () => {
        await this.container.createIfNotExists();
      })();
    }
    await this.ensureContainerPromise;
  }

  async get(key: string): Promise<string | null> {
    await this.ensureContainer();
    const blob = this.container.getBlockBlobClient(key);
    try {
      const response = await blob.download();
      return streamToUtf8(response.readableStreamBody);
    } catch (error) {
      if (isHttpStatusError(error, 404) || isErrnoWithCode(error, "BlobNotFound")) {
        return null;
      }
      throw error;
    }
  }

  async put(key: string, value: string): Promise<void> {
    await this.ensureContainer();
    const blob = this.container.getBlockBlobClient(key);
    await blob.upload(value, Buffer.byteLength(value), {
      blobHTTPHeaders: {
        blobContentType: "application/json; charset=utf-8"
      }
    });
  }

  async putIfAbsent(key: string, value: string): Promise<boolean> {
    await this.ensureContainer();
    const blob = this.container.getBlockBlobClient(key);
    try {
      await blob.upload(value, Buffer.byteLength(value), {
        blobHTTPHeaders: {
          blobContentType: "application/json; charset=utf-8"
        },
        conditions: {
          ifNoneMatch: "*"
        }
      });
      return true;
    } catch (error) {
      if (isHttpStatusError(error, 412) || isErrnoWithCode(error, "ConditionNotMet")) {
        return false;
      }
      throw error;
    }
  }

  async delete(key: string): Promise<void> {
    await this.ensureContainer();
    await this.container.getBlockBlobClient(key).deleteIfExists();
  }

  async list(prefix: string): Promise<string[]> {
    await this.ensureContainer();
    const out: string[] = [];
    for await (const blob of this.container.listBlobsFlat({ prefix })) {
      if (blob && typeof blob.name === "string") {
        out.push(blob.name);
      }
    }
    return out;
  }
}

class PostgresStateBackend extends KeyValueStateBackend {
  constructor(config: ResolvedStateConfig) {
    super({
      backend: "postgres",
      config,
      keyPrefix: normalizeBackendKeyPrefix(config),
      store: new PostgresKeyValueStore(config)
    });
  }
}

class S3StateBackend extends KeyValueStateBackend {
  constructor(config: ResolvedStateConfig) {
    super({
      backend: "s3",
      config,
      keyPrefix: normalizeBackendKeyPrefix(config),
      store: new S3KeyValueStore(config)
    });
  }
}

class GcsStateBackend extends KeyValueStateBackend {
  constructor(config: ResolvedStateConfig) {
    super({
      backend: "gcs",
      config,
      keyPrefix: normalizeBackendKeyPrefix(config),
      store: new GcsKeyValueStore(config)
    });
  }
}

class AzBlobStateBackend extends KeyValueStateBackend {
  constructor(config: ResolvedStateConfig) {
    super({
      backend: "azblob",
      config,
      keyPrefix: normalizeBackendKeyPrefix(config),
      store: new AzBlobKeyValueStore(config)
    });
  }
}

export function createStateBackend(options: StateBackendFactoryOptions): StateBackend {
  const stateConfig = normalizeStateConfig(backendFromState(options.state, options.backendOverride));
  switch (stateConfig.backend) {
    case "local":
      return new LocalFileStateBackend({
        projectDir: options.projectDir,
        state: stateConfig
      });
    case "postgres":
      return new PostgresStateBackend(stateConfig);
    case "s3":
      return new S3StateBackend(stateConfig);
    case "gcs":
      return new GcsStateBackend(stateConfig);
    case "azblob":
      return new AzBlobStateBackend(stateConfig);
    default:
      return new LocalFileStateBackend({
        projectDir: options.projectDir,
        state: { ...stateConfig, backend: "local" }
      });
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

  const records = Array.isArray(parsed.records)
    ? parsed.records.map((entry) => {
        if (!isRecord(entry) || !isRecord(entry.address)) {
          throw new Error("state backup record entry is invalid");
        }
        return {
          address: {
            service: String(entry.address.service || ""),
            stage: String(entry.address.stage || ""),
            provider: String(entry.address.provider || "")
          },
          record: migrateStateRecord(entry.record)
        } as StateRecordEntry;
      })
    : [];

  const locks = Array.isArray(parsed.locks)
    ? parsed.locks
        .map((entry) => {
          if (!isRecord(entry) || !isRecord(entry.address) || !isRecord(entry.lock)) {
            return null;
          }
          return {
            address: {
              service: String(entry.address.service || ""),
              stage: String(entry.address.stage || ""),
              provider: String(entry.address.provider || "")
            },
            lock: {
              backend:
                typeof entry.lock.backend === "string" &&
                ["local", "postgres", "s3", "gcs", "azblob"].includes(entry.lock.backend)
                  ? (entry.lock.backend as StateBackendType)
                  : "local",
              lockId: String(entry.lock.lockId || ""),
              owner: String(entry.lock.owner || ""),
              acquiredAt: String(entry.lock.acquiredAt || toIsoNow()),
              heartbeatAt:
                typeof entry.lock.heartbeatAt === "string" ? entry.lock.heartbeatAt : undefined,
              expiresAt: String(entry.lock.expiresAt || toIsoNow())
            } as StateLockInfo
          } as StateLockEntry;
        })
        .filter((entry): entry is StateLockEntry => !!entry)
    : [];

  return {
    schemaVersion:
      typeof parsed.schemaVersion === "number" && Number.isFinite(parsed.schemaVersion)
        ? parsed.schemaVersion
        : CURRENT_STATE_SCHEMA_VERSION,
    createdAt: typeof parsed.createdAt === "string" ? parsed.createdAt : toIsoNow(),
    backend:
      typeof parsed.backend === "string" &&
      ["local", "postgres", "s3", "gcs", "azblob"].includes(parsed.backend)
        ? (parsed.backend as StateBackendType)
        : "local",
    records,
    locks
  };
}
