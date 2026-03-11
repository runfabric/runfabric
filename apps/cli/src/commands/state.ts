import type { CommandRegistrar } from "../types/cli";
import { readdir, readFile } from "node:fs/promises";
import { resolve } from "node:path";
import {
  createStateBackend,
  migrateStateBetweenBackends,
  readStateBackupFile,
  stateAddressToKey,
  type StateBackend,
  type StateBackendType,
  type StateListFilter,
  writeStateBackupFile
} from "@runfabric/core";
import { loadPlanningContext } from "../utils/load-config";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { info, warn } from "../utils/logger";

interface StateContext {
  projectDir: string;
  service: string;
  stage: string;
  backend: StateBackend;
}

function parseBackend(value: string): StateBackendType {
  const normalized = value.trim().toLowerCase();
  if (!["local", "postgres", "s3", "gcs", "azblob"].includes(normalized)) {
    throw new Error("backend must be one of: local, postgres, s3, gcs, azblob");
  }
  return normalized as StateBackendType;
}

function parseOptionalBackend(value?: string): StateBackendType | undefined {
  return value ? parseBackend(value) : undefined;
}

function toFilter(
  service: string | undefined,
  stage: string | undefined,
  provider: string | undefined
): StateListFilter {
  return {
    service: service?.trim() || undefined,
    stage: stage?.trim() || undefined,
    provider: provider?.trim() || undefined
  };
}

async function loadStateContext(options: {
  config?: string;
  stage?: string;
  backendOverride?: StateBackendType;
}): Promise<StateContext> {
  const projectDir = await resolveProjectDir(process.cwd(), options.config);
  const configPath = options.config ? resolve(process.cwd(), options.config) : undefined;
  const context = await loadPlanningContext(projectDir, configPath, options.stage);
  const stage = context.project.stage || "default";
  const backend = createStateBackend({
    projectDir,
    state: context.project.state,
    backendOverride: options.backendOverride
  });

  return {
    projectDir,
    service: context.project.service,
    stage,
    backend
  };
}

async function readDeploymentReceipt(
  projectDir: string,
  provider: string
): Promise<Record<string, unknown> | null> {
  const receiptPath = resolve(projectDir, ".runfabric", "deploy", provider, "deployment.json");
  try {
    return JSON.parse(await readFile(receiptPath, "utf8")) as Record<string, unknown>;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") {
      return null;
    }
    throw error;
  }
}

export const registerStateCommand: CommandRegistrar = (program) => {
  const state = program.command("state").description("State backend operations and diagnostics");

  state
    .command("pull")
    .description("Read one provider state record")
    .requiredOption("-p, --provider <name>", "Provider name")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-b, --backend <name>", "Backend override (local|postgres|s3|gcs|azblob)")
    .option("-s, --stage <name>", "Stage name override")
    .option("--service <name>", "Service name override")
    .option("--json", "Emit JSON output")
    .action(
      async (options: {
        provider: string;
        config?: string;
        backend?: string;
        stage?: string;
        service?: string;
        json?: boolean;
      }) => {
        const context = await loadStateContext({
          config: options.config,
          stage: options.stage,
          backendOverride: parseOptionalBackend(options.backend)
        });
        const address = {
          service: options.service || context.service,
          stage: context.stage,
          provider: options.provider
        };
        const record = await context.backend.read(address);
        const lock = await context.backend.readLock(address);
        const payload = {
          backend: context.backend.backend,
          address,
          record,
          lock
        };

        if (!record) {
          process.exitCode = 1;
        }

        if (options.json) {
          printJson(payload);
        } else if (!record) {
          warn(`state record not found for ${stateAddressToKey(address)}`);
        } else {
          info(`state ${stateAddressToKey(address)} lifecycle=${record.lifecycle}`);
          info(`updatedAt=${record.updatedAt}`);
          info(`endpoint=${record.endpoint || "n/a"}`);
          if (lock) {
            info(`lock owner=${lock.owner} lockId=${lock.lockId} expiresAt=${lock.expiresAt}`);
          }
        }
      }
    );

  state
    .command("list")
    .description("List state records and lock metadata")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-b, --backend <name>", "Backend override (local|postgres|s3|gcs|azblob)")
    .option("-s, --stage <name>", "Stage name override")
    .option("--service <name>", "Filter by service")
    .option("-p, --provider <name>", "Filter by provider")
    .option("--json", "Emit JSON output")
    .action(
      async (options: {
        config?: string;
        backend?: string;
        stage?: string;
        service?: string;
        provider?: string;
        json?: boolean;
      }) => {
        const context = await loadStateContext({
          config: options.config,
          stage: options.stage,
          backendOverride: parseOptionalBackend(options.backend)
        });
        const filter = toFilter(options.service || context.service, context.stage, options.provider);
        const records = await context.backend.list(filter);
        const locks = await context.backend.listLocks(filter);
        const payload = {
          backend: context.backend.backend,
          filter,
          records,
          locks
        };

        if (options.json) {
          printJson(payload);
        } else {
          info(`backend=${payload.backend} records=${records.length} locks=${locks.length}`);
          for (const entry of records) {
            info(
              `${stateAddressToKey(entry.address)} lifecycle=${entry.record.lifecycle} endpoint=${entry.record.endpoint || "n/a"}`
            );
          }
          for (const entry of locks) {
            info(
              `lock ${stateAddressToKey(entry.address)} owner=${entry.lock.owner} expiresAt=${entry.lock.expiresAt}`
            );
          }
        }
      }
    );

  state
    .command("backup")
    .description("Create state backup snapshot")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-b, --backend <name>", "Backend override (local|postgres|s3|gcs|azblob)")
    .option("-s, --stage <name>", "Stage name override")
    .option("--service <name>", "Filter by service")
    .option("-p, --provider <name>", "Filter by provider")
    .option("-o, --out <path>", "Output backup file path")
    .option("--json", "Emit JSON output")
    .action(
      async (options: {
        config?: string;
        backend?: string;
        stage?: string;
        service?: string;
        provider?: string;
        out?: string;
        json?: boolean;
      }) => {
        const context = await loadStateContext({
          config: options.config,
          stage: options.stage,
          backendOverride: parseOptionalBackend(options.backend)
        });
        const filter = toFilter(options.service || context.service, context.stage, options.provider);
        const backup = await context.backend.createBackup(filter);
        const outPath =
          options.out ||
          resolve(
            context.projectDir,
            ".runfabric",
            "backup",
            `state-${context.backend.backend}-${new Date().toISOString().replace(/[:.]/g, "-")}.json`
          );
        await writeStateBackupFile(resolve(process.cwd(), outPath), backup);

        const payload = {
          backend: context.backend.backend,
          out: resolve(process.cwd(), outPath),
          records: backup.records.length,
          locks: backup.locks.length,
          createdAt: backup.createdAt
        };

        if (options.json) {
          printJson(payload);
        } else {
          info(`state backup written: ${payload.out}`);
          info(`records=${payload.records} locks=${payload.locks}`);
        }
      }
    );

  state
    .command("restore")
    .description("Restore state snapshot from backup file")
    .requiredOption("-f, --file <path>", "Backup file path")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-b, --backend <name>", "Backend override (local|postgres|s3|gcs|azblob)")
    .option("-s, --stage <name>", "Stage name override")
    .option("--json", "Emit JSON output")
    .action(
      async (options: { file: string; config?: string; backend?: string; stage?: string; json?: boolean }) => {
        const context = await loadStateContext({
          config: options.config,
          stage: options.stage,
          backendOverride: parseOptionalBackend(options.backend)
        });
        const backup = await readStateBackupFile(resolve(process.cwd(), options.file));
        await context.backend.restoreBackup(backup);

        const payload = {
          backend: context.backend.backend,
          file: resolve(process.cwd(), options.file),
          restoredRecords: backup.records.length,
          restoredLocks: backup.locks.length
        };

        if (options.json) {
          printJson(payload);
        } else {
          info(`state restore complete from ${payload.file}`);
          info(`records=${payload.restoredRecords} locks=${payload.restoredLocks}`);
        }
      }
    );

  state
    .command("force-unlock")
    .description("Force unlock provider state lock")
    .requiredOption("-p, --provider <name>", "Provider name")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-b, --backend <name>", "Backend override (local|postgres|s3|gcs|azblob)")
    .option("-s, --stage <name>", "Stage name override")
    .option("--service <name>", "Service name override")
    .option("--json", "Emit JSON output")
    .action(
      async (options: {
        provider: string;
        config?: string;
        backend?: string;
        stage?: string;
        service?: string;
        json?: boolean;
      }) => {
        const context = await loadStateContext({
          config: options.config,
          stage: options.stage,
          backendOverride: parseOptionalBackend(options.backend)
        });
        const address = {
          service: options.service || context.service,
          stage: context.stage,
          provider: options.provider
        };
        const removed = await context.backend.forceUnlock(address);
        const payload = {
          backend: context.backend.backend,
          address,
          removed
        };

        if (options.json) {
          printJson(payload);
        } else if (removed) {
          info(`force-unlock removed lock for ${stateAddressToKey(address)}`);
        } else {
          warn(`no lock found for ${stateAddressToKey(address)}`);
        }
      }
    );

  state
    .command("migrate")
    .description("Migrate state records between backends")
    .requiredOption("--from <backend>", "Source backend")
    .requiredOption("--to <backend>", "Target backend")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-b, --backend <name>", "Default backend context override")
    .option("-s, --stage <name>", "Stage name override")
    .option("--service <name>", "Filter by service")
    .option("-p, --provider <name>", "Filter by provider")
    .option("--json", "Emit JSON output")
    .action(
      async (options: {
        from: string;
        to: string;
        config?: string;
        backend?: string;
        stage?: string;
        service?: string;
        provider?: string;
        json?: boolean;
      }) => {
        const fromBackendType = parseBackend(options.from);
        const toBackendType = parseBackend(options.to);
        if (fromBackendType === toBackendType) {
          throw new Error("--from and --to must differ");
        }

        const context = await loadStateContext({
          config: options.config,
          stage: options.stage,
          backendOverride: parseOptionalBackend(options.backend)
        });
        const filter = toFilter(options.service || context.service, context.stage, options.provider);

        const source = createStateBackend({
          projectDir: context.projectDir,
          state: context.backend.config,
          backendOverride: fromBackendType
        });
        const target = createStateBackend({
          projectDir: context.projectDir,
          state: context.backend.config,
          backendOverride: toBackendType
        });

        const result = await migrateStateBetweenBackends(source, target, filter);
        const payload = {
          from: fromBackendType,
          to: toBackendType,
          copiedCount: result.copiedCount,
          fromChecksum: result.fromChecksum,
          toChecksum: result.toChecksum
        };

        if (options.json) {
          printJson(payload);
        } else {
          info(`state migrate ${fromBackendType} -> ${toBackendType} copied=${payload.copiedCount}`);
          info(`checksum=${payload.toChecksum}`);
        }
      }
    );

  state
    .command("reconcile")
    .description("Compare state records against deployment receipts")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-b, --backend <name>", "Backend override (local|postgres|s3|gcs|azblob)")
    .option("-s, --stage <name>", "Stage name override")
    .option("--service <name>", "Service name override")
    .option("-p, --provider <name>", "Filter by provider")
    .option("--json", "Emit JSON output")
    .action(
      async (options: {
        config?: string;
        backend?: string;
        stage?: string;
        service?: string;
        provider?: string;
        json?: boolean;
      }) => {
        const context = await loadStateContext({
          config: options.config,
          stage: options.stage,
          backendOverride: parseOptionalBackend(options.backend)
        });
        const service = options.service || context.service;
        const filter = toFilter(service, context.stage, options.provider);
        const records = await context.backend.list(filter);
        const locks = await context.backend.listLocks(filter);
        const missingReceipt: string[] = [];
        const endpointMismatch: Array<{
          key: string;
          stateEndpoint?: string;
          receiptEndpoint?: string;
        }> = [];

        for (const entry of records) {
          const key = stateAddressToKey(entry.address);
          const receipt = await readDeploymentReceipt(context.projectDir, entry.address.provider);
          if (!receipt) {
            missingReceipt.push(key);
            continue;
          }
          const receiptEndpoint =
            typeof receipt.endpoint === "string" ? receipt.endpoint : undefined;
          if (entry.record.endpoint !== receiptEndpoint) {
            endpointMismatch.push({
              key,
              stateEndpoint: entry.record.endpoint,
              receiptEndpoint
            });
          }
        }

        const missingState: string[] = [];
        const deployRoot = resolve(context.projectDir, ".runfabric", "deploy");
        try {
          const providers = await readdir(deployRoot, { withFileTypes: true });
          const knownStateKeys = new Set(records.map((entry) => stateAddressToKey(entry.address)));
          for (const providerDir of providers) {
            if (!providerDir.isDirectory()) {
              continue;
            }
            const provider = providerDir.name;
            if (options.provider && options.provider !== provider) {
              continue;
            }
            const receipt = await readDeploymentReceipt(context.projectDir, provider);
            if (!receipt) {
              continue;
            }
            const receiptService =
              typeof receipt.service === "string" ? receipt.service : service;
            const receiptStage =
              typeof receipt.stage === "string" ? receipt.stage : context.stage;
            const key = stateAddressToKey({
              service: receiptService,
              stage: receiptStage,
              provider
            });
            if (!knownStateKeys.has(key)) {
              missingState.push(key);
            }
          }
        } catch (readError) {
          if ((readError as NodeJS.ErrnoException).code !== "ENOENT") {
            throw readError;
          }
        }

        const driftCount = missingReceipt.length + endpointMismatch.length + missingState.length;
        if (driftCount > 0) {
          process.exitCode = 2;
        }

        const payload = {
          backend: context.backend.backend,
          filter,
          summary: {
            records: records.length,
            locks: locks.length,
            driftCount
          },
          drift: {
            missingReceipt,
            endpointMismatch,
            missingState
          }
        };

        if (options.json) {
          printJson(payload);
        } else {
          info(
            `state reconcile backend=${payload.backend} records=${records.length} locks=${locks.length} drift=${driftCount}`
          );
          for (const key of missingReceipt) {
            warn(`missing receipt for ${key}`);
          }
          for (const mismatch of endpointMismatch) {
            warn(
              `endpoint mismatch ${mismatch.key}: state=${mismatch.stateEndpoint || "n/a"} receipt=${mismatch.receiptEndpoint || "n/a"}`
            );
          }
          for (const key of missingState) {
            warn(`missing state for receipt ${key}`);
          }
          if (driftCount === 0) {
            info("state and receipts are consistent");
          }
        }
      }
    );
};
