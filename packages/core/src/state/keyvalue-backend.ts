import { randomUUID } from "node:crypto";
import type {
  ProviderStateRecord,
  ResolvedStateConfig,
  StateAddress,
  StateBackend,
  StateBackupSnapshot,
  StateListFilter,
  StateLockEntry,
  StateLockInfo,
  StateRecordEntry
} from "../state";
import type { StateBackendType } from "../project";
import {
  AzBlobKeyValueStore,
  GcsKeyValueStore,
  type KeyValueStore,
  PostgresKeyValueStore,
  S3KeyValueStore
} from "./key-value-stores";
import {
  CURRENT_STATE_SCHEMA_VERSION,
  LOCK_FILE_SUFFIX,
  STATE_FILE_SUFFIX,
  isRecord,
  migrateStateRecord,
  normalizeRecord,
  resolveAddress,
  toIsoNow
} from "./record-utils";

const STATE_LIST_READ_CONCURRENCY = 8;

function normalizeObjectPrefix(value: string): string {
  return value
    .split("/")
    .map((segment) => segment.trim())
    .filter(Boolean)
    .join("/");
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

export class KeyValueStateBackend implements StateBackend {
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
    return provider ? { service, stage, provider } : null;
  }

  private matchesFilter(address: StateAddress, record: ProviderStateRecord, filter?: StateListFilter): boolean {
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

  private disabledLockInfo(): StateLockInfo {
    const now = toIsoNow();
    return {
      backend: this.backend,
      lockId: "locks-disabled",
      owner: "locks-disabled",
      acquiredAt: now,
      heartbeatAt: now,
      expiresAt: now
    };
  }

  private createLockInfo(owner: string): StateLockInfo {
    const acquiredAt = toIsoNow();
    return {
      backend: this.backend,
      lockId: randomUUID(),
      owner,
      acquiredAt,
      heartbeatAt: acquiredAt,
      expiresAt: new Date(Date.now() + this.config.lock.timeoutSeconds * 1000).toISOString()
    };
  }

  private async tryAcquireLock(lockKey: string, lockInfo: StateLockInfo): Promise<boolean> {
    const serialized = JSON.stringify(lockInfo, null, 2);

    if (this.store.putIfAbsent) {
      return this.store.putIfAbsent(lockKey, serialized);
    }

    const existing = await this.store.get(lockKey);
    if (existing) {
      return false;
    }

    await this.store.put(lockKey, serialized);
    const verify = await this.readLockByKey(lockKey);
    return Boolean(verify && verify.lockId === lockInfo.lockId);
  }

  private async readLockByKey(lockKey: string): Promise<StateLockInfo | null> {
    const content = await this.store.get(lockKey);
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

  private parseStateRecordContent(content: string, key: string): ProviderStateRecord {
    try {
      return migrateStateRecord(JSON.parse(content) as unknown);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      throw new Error(`failed to parse state record at ${key}: ${message}`);
    }
  }

  async read(address: StateAddress): Promise<ProviderStateRecord | null> {
    const stateKey = this.toStateKey(address);
    const content = await this.store.get(stateKey);
    if (!content) {
      return null;
    }
    return this.parseStateRecordContent(content, stateKey);
  }

  async write(address: StateAddress, state: ProviderStateRecord, lock?: StateLockInfo): Promise<void> {
    if (this.config.lock.enabled) {
      const currentLock = await this.readLock(address);
      if (currentLock && (!lock || lock.lockId !== currentLock.lockId)) {
        throw this.lockError(address, currentLock);
      }
    }

    await this.store.put(this.toStateKey(address), JSON.stringify(normalizeRecord(state), null, 2));
  }

  async delete(address: StateAddress): Promise<void> {
    await this.store.delete(this.toStateKey(address));
  }

  private async readListEntryByKey(key: string, filter?: StateListFilter): Promise<StateRecordEntry | null> {
    const address = this.extractAddressFromKey(key);
    if (!address) {
      return null;
    }

    let content: string | null;
    try {
      content = await this.store.get(key);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      throw new Error(`failed to read state record at ${key}: ${message}`);
    }

    if (!content) {
      return null;
    }

    const record = this.parseStateRecordContent(content, key);
    return this.matchesFilter(address, record, filter) ? { address, record } : null;
  }

  async list(filter?: StateListFilter): Promise<StateRecordEntry[]> {
    const keys = await this.store.list(this.objectPrefix());
    const stateKeys = keys.filter((key) => key.endsWith(STATE_FILE_SUFFIX) && !key.endsWith(LOCK_FILE_SUFFIX));
    if (stateKeys.length === 0) {
      return [];
    }

    const entries: Array<StateRecordEntry | null> = new Array(stateKeys.length);
    const workerCount = Math.min(STATE_LIST_READ_CONCURRENCY, stateKeys.length);
    let nextIndex = 0;

    const workers = Array.from({ length: workerCount }, async () => {
      while (true) {
        const currentIndex = nextIndex;
        nextIndex += 1;
        if (currentIndex >= stateKeys.length) {
          return;
        }
        entries[currentIndex] = await this.readListEntryByKey(stateKeys[currentIndex], filter);
      }
    });

    await Promise.all(workers);
    return entries.filter((entry): entry is StateRecordEntry => entry !== null);
  }

  async lock(address: StateAddress, owner = `pid:${process.pid}`): Promise<StateLockInfo> {
    if (!this.config.lock.enabled) {
      return this.disabledLockInfo();
    }

    const lockKey = this.toLockKey(address);
    for (let attempt = 0; attempt < 2; attempt += 1) {
      const lockInfo = this.createLockInfo(owner);
      if (await this.tryAcquireLock(lockKey, lockInfo)) {
        return lockInfo;
      }

      const existing = await this.readLockByKey(lockKey);
      if (existing && this.isStale(existing)) {
        await this.store.delete(lockKey);
        continue;
      }

      throw this.lockError(address, existing);
    }

    throw this.lockError(address, await this.readLock(address));
  }

  async readLock(address: StateAddress): Promise<StateLockInfo | null> {
    return this.readLockByKey(this.toLockKey(address));
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

    const refreshed: StateLockInfo = {
      ...existing,
      heartbeatAt: toIsoNow(),
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
      if (lock) {
        entries.push({ address, lock });
      }
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

export class PostgresStateBackend extends KeyValueStateBackend {
  constructor(config: ResolvedStateConfig) {
    super({
      backend: "postgres",
      config,
      keyPrefix: normalizeBackendKeyPrefix(config),
      store: new PostgresKeyValueStore(config)
    });
  }
}

export class S3StateBackend extends KeyValueStateBackend {
  constructor(config: ResolvedStateConfig) {
    super({
      backend: "s3",
      config,
      keyPrefix: normalizeBackendKeyPrefix(config),
      store: new S3KeyValueStore(config)
    });
  }
}

export class GcsStateBackend extends KeyValueStateBackend {
  constructor(config: ResolvedStateConfig) {
    super({
      backend: "gcs",
      config,
      keyPrefix: normalizeBackendKeyPrefix(config),
      store: new GcsKeyValueStore(config)
    });
  }
}

export class AzBlobStateBackend extends KeyValueStateBackend {
  constructor(config: ResolvedStateConfig) {
    super({
      backend: "azblob",
      config,
      keyPrefix: normalizeBackendKeyPrefix(config),
      store: new AzBlobKeyValueStore(config)
    });
  }
}
