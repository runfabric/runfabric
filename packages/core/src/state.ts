import { mkdir, open, readFile, rm, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

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
  endpoint?: string;
  details?: Record<string, unknown>;
}

export interface StateBackend {
  read(address: StateAddress): Promise<ProviderStateRecord | null>;
  write(address: StateAddress, state: ProviderStateRecord): Promise<void>;
  lock(address: StateAddress): Promise<void>;
  unlock(address: StateAddress): Promise<void>;
}

export interface LocalFileStateBackendOptions {
  projectDir: string;
}

export class LocalFileStateBackend implements StateBackend {
  private readonly rootDir: string;

  constructor(options: LocalFileStateBackendOptions) {
    this.rootDir = resolve(options.projectDir, ".runfabric", "state");
  }

  private stateFile(address: StateAddress): string {
    return resolve(this.rootDir, address.service, address.stage, `${address.provider}.state.json`);
  }

  private lockFile(address: StateAddress): string {
    return `${this.stateFile(address)}.lock`;
  }

  async read(address: StateAddress): Promise<ProviderStateRecord | null> {
    const filePath = this.stateFile(address);
    try {
      const content = await readFile(filePath, "utf8");
      const parsed = JSON.parse(content) as ProviderStateRecord;
      return parsed;
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") {
        return null;
      }
      throw error;
    }
  }

  async write(address: StateAddress, state: ProviderStateRecord): Promise<void> {
    const filePath = this.stateFile(address);
    await mkdir(resolve(filePath, ".."), { recursive: true });
    await writeFile(filePath, JSON.stringify(state, null, 2), "utf8");
  }

  async lock(address: StateAddress): Promise<void> {
    const filePath = this.stateFile(address);
    await mkdir(resolve(filePath, ".."), { recursive: true });
    const lockPath = this.lockFile(address);
    const handle = await open(lockPath, "wx");
    await handle.close();
  }

  async unlock(address: StateAddress): Promise<void> {
    const lockPath = this.lockFile(address);
    try {
      await rm(lockPath);
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "ENOENT") {
        throw error;
      }
    }
  }
}
