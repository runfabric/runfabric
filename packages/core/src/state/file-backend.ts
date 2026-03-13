import { randomUUID } from "node:crypto";
import { mkdir, open, readdir, readFile, rm, writeFile } from "node:fs/promises";
import { dirname, isAbsolute, relative, resolve } from "node:path";
import type {
  LocalFileStateBackendOptions,
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
  isResolvedStateConfig,
  normalizeStateConfig,
  resolveBackendRootDir
} from "./config-utils";
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

class FileStateBackend implements StateBackend {
  readonly backend: StateBackendType;
  readonly config: ResolvedStateConfig;
  private readonly rootDir: string;

  constructor(options: { backend: StateBackendType; config: ResolvedStateConfig; rootDir: string }) {
    this.backend = options.backend;
    this.config = options.config;
    this.rootDir = options.rootDir;
  }

  private resolveWithinRoot(...segments: string[]): string {
    const candidate = resolve(this.rootDir, ...segments);
    const relativePath = relative(this.rootDir, candidate);
    if (relativePath === "" || (!relativePath.startsWith("..") && !isAbsolute(relativePath))) {
      return candidate;
    }
    throw new Error(`state address escapes configured backend root: ${segments.join("/")}`);
  }

  private toStateFilePath(address: StateAddress): string {
    const safeAddress = resolveAddress(address);
    return this.resolveWithinRoot(
      safeAddress.service,
      safeAddress.stage,
      `${safeAddress.provider}${STATE_FILE_SUFFIX}`
    );
  }

  private toLockFilePath(address: StateAddress): string {
    return `${this.toStateFilePath(address)}.lock`;
  }

  private async ensureParent(path: string): Promise<void> {
    await mkdir(dirname(path), { recursive: true });
  }

  private async listFilesBySuffix(suffix: string): Promise<string[]> {
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
            if (file.isFile() && file.name.endsWith(suffix)) {
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

  private listStateFiles(): Promise<string[]> {
    return this.listFilesBySuffix(STATE_FILE_SUFFIX);
  }

  private listLockFiles(): Promise<string[]> {
    return this.listFilesBySuffix(LOCK_FILE_SUFFIX);
  }

  private extractAddressFromPath(path: string): StateAddress | null {
    const relativePath = relative(this.rootDir, path);
    if (!relativePath || relativePath.startsWith("..") || isAbsolute(relativePath)) {
      return null;
    }

    const segments = relativePath.split(/[\\/]+/);
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

  private parseStateRecordContent(content: string, source: string): ProviderStateRecord {
    try {
      return migrateStateRecord(JSON.parse(content) as unknown);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      throw new Error(`failed to parse state record at ${source}: ${message}`);
    }
  }

  async read(address: StateAddress): Promise<ProviderStateRecord | null> {
    const filePath = this.toStateFilePath(address);
    try {
      const content = await readFile(filePath, "utf8");
      return this.parseStateRecordContent(content, filePath);
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
      if (currentLock && (!lock || lock.lockId !== currentLock.lockId)) {
        throw this.lockError(address, currentLock);
      }
    }

    await writeFile(filePath, JSON.stringify(normalizeRecord(state), null, 2), "utf8");
  }

  async delete(address: StateAddress): Promise<void> {
    await rm(this.toStateFilePath(address), { force: true });
  }

  private async readListEntry(filePath: string, filter?: StateListFilter): Promise<StateRecordEntry | null> {
    const address = this.extractAddressFromPath(filePath);
    if (!address) {
      return null;
    }

    let content: string;
    try {
      content = await readFile(filePath, "utf8");
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") {
        return null;
      }
      const message = error instanceof Error ? error.message : String(error);
      throw new Error(`failed to read state record at ${filePath}: ${message}`);
    }

    const record = this.parseStateRecordContent(content, filePath);
    return this.matchesFilter(address, record, filter) ? { address, record } : null;
  }

  async list(filter?: StateListFilter): Promise<StateRecordEntry[]> {
    const files = await this.listStateFiles();
    if (files.length === 0) {
      return [];
    }

    const entries: Array<StateRecordEntry | null> = new Array(files.length);
    const workerCount = Math.min(STATE_LIST_READ_CONCURRENCY, files.length);
    let nextIndex = 0;

    const workers = Array.from({ length: workerCount }, async () => {
      while (true) {
        const currentIndex = nextIndex;
        nextIndex += 1;
        if (currentIndex >= files.length) {
          return;
        }
        entries[currentIndex] = await this.readListEntry(files[currentIndex], filter);
      }
    });

    await Promise.all(workers);
    return entries.filter((entry): entry is StateRecordEntry => entry !== null);
  }

  async lock(address: StateAddress, owner = `pid:${process.pid}`): Promise<StateLockInfo> {
    if (!this.config.lock.enabled) {
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

      return {
        backend: this.backend,
        lockId: typeof parsed.lockId === "string" ? parsed.lockId : "unknown",
        owner: typeof parsed.owner === "string" ? parsed.owner : "unknown",
        acquiredAt: typeof parsed.acquiredAt === "string" ? parsed.acquiredAt : toIsoNow(),
        heartbeatAt: typeof parsed.heartbeatAt === "string" ? parsed.heartbeatAt : undefined,
        expiresAt: typeof parsed.expiresAt === "string" ? parsed.expiresAt : toIsoNow()
      };
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
    await writeFile(this.toLockFilePath(address), JSON.stringify(refreshed, null, 2), "utf8");
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

    await rm(this.toLockFilePath(address), { force: true });
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
      const lockPath = this.toLockFilePath(entry.address);
      await this.ensureParent(lockPath);
      await writeFile(lockPath, JSON.stringify(entry.lock, null, 2), "utf8");
    }
  }
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
