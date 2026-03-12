import { access, readFile, writeFile } from "node:fs/promises";
import { constants } from "node:fs";
import { dirname, resolve } from "node:path";
import type { CommandRegistrar } from "../types/cli";
import { printJson } from "../utils/output";
import { error, info, success, warn } from "../utils/logger";
import {
  buildRunfabricYaml,
  migrateServerlessToRunfabric,
  type MigrationDraft
} from "./migrate/parser";

interface MigrateCommandOptions {
  input: string;
  output?: string;
  provider?: string;
  dryRun?: boolean;
  force?: boolean;
  json?: boolean;
}

function resolveMigrationOutputPath(inputPath: string, outputOverride?: string): string {
  if (outputOverride && outputOverride.trim().length > 0) {
    return resolve(process.cwd(), outputOverride);
  }
  return resolve(dirname(inputPath), "runfabric.yml");
}

async function assertMigrationOutputWritable(outputPath: string, force?: boolean): Promise<void> {
  if (force) {
    return;
  }
  try {
    await access(outputPath, constants.F_OK);
    throw new Error(`output file already exists: ${outputPath}. use --force to overwrite`);
  } catch (accessError) {
    if ((accessError as NodeJS.ErrnoException).code !== "ENOENT") {
      throw accessError;
    }
  }
}

function emitMigrationPayload(
  options: MigrateCommandOptions,
  migrated: MigrationDraft,
  inputPath: string,
  outputPath: string
): void {
  const payload = {
    input: inputPath,
    output: options.dryRun ? null : outputPath,
    provider: migrated.provider,
    service: migrated.service,
    runtime: migrated.runtime,
    functionCount: migrated.functions.length,
    triggerCount: migrated.topLevelTriggers.length,
    warnings: migrated.warnings
  };

  if (options.json) {
    printJson(payload);
    return;
  }

  if (!options.dryRun) {
    success(`migrated ${inputPath} -> ${outputPath}`);
  }
  info(`provider=${payload.provider} functions=${payload.functionCount} triggers=${payload.triggerCount}`);
  for (const warningMessage of migrated.warnings) {
    warn(`migration warning: ${warningMessage}`);
  }
}

async function runMigrateCommand(options: MigrateCommandOptions): Promise<void> {
  const inputPath = resolve(process.cwd(), options.input);
  const source = await readFile(inputPath, "utf8");
  const migrated = migrateServerlessToRunfabric(source, options.provider);
  const outputPath = resolveMigrationOutputPath(inputPath, options.output);
  const yaml = buildRunfabricYaml(migrated);

  if (options.dryRun) {
    info(yaml.trimEnd());
  } else {
    await assertMigrationOutputWritable(outputPath, options.force);
    await writeFile(outputPath, yaml, "utf8");
  }

  emitMigrationPayload(options, migrated, inputPath, outputPath);
}

export const registerMigrateCommand: CommandRegistrar = (program) => {
  program
    .command("migrate")
    .description("Best-effort migration from serverless.yml to runfabric.yml")
    .requiredOption("-i, --input <path>", "Path to serverless.yml")
    .option("-o, --output <path>", "Output path for runfabric.yml")
    .option("-p, --provider <name>", "Override target provider id")
    .option("--dry-run", "Print migrated runfabric.yml instead of writing file")
    .option("--force", "Overwrite output file if it exists")
    .option("--json", "Emit JSON summary")
    .action(async (options: MigrateCommandOptions) => {
      try {
        await runMigrateCommand(options);
      } catch (migrationError) {
        const message = migrationError instanceof Error ? migrationError.message : String(migrationError);
        error(message);
        process.exitCode = 1;
      }
    });
};
