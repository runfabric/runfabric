import { access, readFile, writeFile } from "node:fs/promises";
import { constants } from "node:fs";
import { dirname, resolve } from "node:path";
import type { CommandRegistrar } from "../types/cli";
import { printJson } from "../utils/output";
import { error, info, success, warn } from "../utils/logger";

type TriggerDraft =
  | { type: "http"; method: string; path: string }
  | { type: "cron"; schedule: string }
  | { type: "queue"; queue: string }
  | { type: "storage"; bucket: string; events: string[] };

interface FunctionDraft {
  name: string;
  entry?: string;
  triggers: TriggerDraft[];
}

interface MigrationDraft {
  service: string;
  runtime: string;
  entry: string;
  provider: string;
  functions: FunctionDraft[];
  topLevelTriggers: TriggerDraft[];
  warnings: string[];
  extensions?: Record<string, unknown>;
}

function trimQuotes(value: string): string {
  const trimmed = value.trim();
  if (
    (trimmed.startsWith("'") && trimmed.endsWith("'")) ||
    (trimmed.startsWith("\"") && trimmed.endsWith("\""))
  ) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

function readInlineScalar(line: string): string | undefined {
  const separator = line.indexOf(":");
  if (separator < 0) {
    return undefined;
  }
  return trimQuotes(line.slice(separator + 1));
}

function leadingSpaces(line: string): number {
  let count = 0;
  for (const char of line) {
    if (char !== " ") {
      break;
    }
    count += 1;
  }
  return count;
}

function parseProviderName(value: string): string {
  const normalized = value.trim().toLowerCase();
  switch (normalized) {
    case "aws":
    case "aws-lambda":
      return "aws-lambda";
    case "google":
    case "gcp":
    case "gcp-functions":
      return "gcp-functions";
    case "azure":
    case "azure-functions":
      return "azure-functions";
    case "cloudflare":
    case "cloudflare-workers":
      return "cloudflare-workers";
    case "vercel":
      return "vercel";
    case "netlify":
      return "netlify";
    case "alibaba":
    case "alibaba-fc":
      return "alibaba-fc";
    case "digitalocean":
    case "digitalocean-functions":
      return "digitalocean-functions";
    case "fly":
    case "fly-machines":
      return "fly-machines";
    case "ibm":
    case "openwhisk":
    case "ibm-openwhisk":
      return "ibm-openwhisk";
    default:
      return "aws-lambda";
  }
}

function normalizeRuntime(runtime: string | undefined, warnings: string[]): string {
  if (!runtime) {
    return "nodejs";
  }

  const normalized = runtime.trim().toLowerCase();
  if (normalized.startsWith("node")) {
    return "nodejs";
  }

  warnings.push(
    `runtime ${runtime} is not currently production-ready in runfabric; migrated runtime set to nodejs`
  );
  return "nodejs";
}

function entryFromHandler(handler: string | undefined): string | undefined {
  if (!handler || handler.trim().length === 0) {
    return undefined;
  }

  const normalized = trimQuotes(handler).trim();
  const pathPart = normalized.includes(".")
    ? normalized.slice(0, normalized.lastIndexOf("."))
    : normalized;

  if (!pathPart) {
    return undefined;
  }
  if (pathPart.endsWith(".ts") || pathPart.endsWith(".js")) {
    return pathPart;
  }
  return `${pathPart}.ts`;
}

function normalizeHttpPath(path: string): string {
  const value = trimQuotes(path).trim();
  if (!value) {
    return "/hello";
  }
  return value.startsWith("/") ? value : `/${value}`;
}

function parseHttpTrigger(raw: string): TriggerDraft | undefined {
  const value = trimQuotes(raw).trim();
  if (!value) {
    return undefined;
  }

  // "GET /path"
  const methodPathMatch = value.match(/^([a-zA-Z]+)\s+(.+)$/);
  if (methodPathMatch) {
    return {
      type: "http",
      method: methodPathMatch[1].toUpperCase(),
      path: normalizeHttpPath(methodPathMatch[2])
    };
  }

  // "/path" fallback
  return {
    type: "http",
    method: "GET",
    path: normalizeHttpPath(value)
  };
}

function readIndentedBlock(lines: string[], startIndex: number, parentIndent: number): string[] {
  const output: string[] = [];
  for (let index = startIndex; index < lines.length; index += 1) {
    const line = lines[index];
    if (line.trim().length === 0) {
      output.push(line);
      continue;
    }
    const indent = leadingSpaces(line);
    if (indent <= parentIndent) {
      break;
    }
    output.push(line);
  }
  return output;
}

function parseEventTrigger(lines: string[], start: number, baseIndent: number): TriggerDraft | undefined {
  const line = lines[start].trim();

  if (line.startsWith("- httpApi:")) {
    return parseHttpTrigger(line.slice("- httpApi:".length));
  }
  if (line.startsWith("- http:")) {
    const inline = line.slice("- http:".length).trim();
    if (inline.length > 0) {
      return parseHttpTrigger(inline);
    }

    const block = readIndentedBlock(lines, start + 1, baseIndent);
    let path = "/hello";
    let method = "GET";
    for (const entry of block) {
      const trimmed = entry.trim();
      if (trimmed.startsWith("path:")) {
        path = normalizeHttpPath(trimmed.slice("path:".length));
      } else if (trimmed.startsWith("method:")) {
        method = trimQuotes(trimmed.slice("method:".length)).toUpperCase();
      }
    }
    return { type: "http", method, path };
  }

  if (line.startsWith("- schedule:")) {
    const inline = trimQuotes(line.slice("- schedule:".length));
    if (inline.length > 0) {
      return { type: "cron", schedule: inline };
    }

    const block = readIndentedBlock(lines, start + 1, baseIndent);
    for (const entry of block) {
      const trimmed = entry.trim();
      if (trimmed.startsWith("rate:")) {
        return { type: "cron", schedule: trimQuotes(trimmed.slice("rate:".length)) };
      }
      if (trimmed.startsWith("cron:")) {
        return { type: "cron", schedule: trimQuotes(trimmed.slice("cron:".length)) };
      }
    }
    return undefined;
  }

  if (line.startsWith("- sqs:")) {
    const inline = trimQuotes(line.slice("- sqs:".length));
    if (inline.length > 0) {
      return { type: "queue", queue: inline };
    }
    const block = readIndentedBlock(lines, start + 1, baseIndent);
    for (const entry of block) {
      const trimmed = entry.trim();
      if (trimmed.startsWith("arn:")) {
        return { type: "queue", queue: trimQuotes(trimmed.slice("arn:".length)) };
      }
      if (trimmed.startsWith("queue:")) {
        return { type: "queue", queue: trimQuotes(trimmed.slice("queue:".length)) };
      }
    }
    return undefined;
  }

  if (line.startsWith("- s3:")) {
    const inline = trimQuotes(line.slice("- s3:".length));
    if (inline.length > 0) {
      return { type: "storage", bucket: inline, events: ["s3:ObjectCreated:*"] };
    }

    const block = readIndentedBlock(lines, start + 1, baseIndent);
    let bucket = "";
    const events: string[] = [];
    for (const entry of block) {
      const trimmed = entry.trim();
      if (trimmed.startsWith("bucket:")) {
        bucket = trimQuotes(trimmed.slice("bucket:".length));
        continue;
      }
      if (trimmed.startsWith("event:")) {
        events.push(trimQuotes(trimmed.slice("event:".length)));
        continue;
      }
      if (trimmed.startsWith("- ") && trimmed.includes("Object")) {
        events.push(trimQuotes(trimmed.slice(2)));
      }
    }
    if (bucket) {
      return {
        type: "storage",
        bucket,
        events: events.length > 0 ? events : ["s3:ObjectCreated:*"]
      };
    }
  }

  return undefined;
}

function parseFunctions(lines: string[], startIndex: number): FunctionDraft[] {
  const output: FunctionDraft[] = [];
  const functionsIndent = leadingSpaces(lines[startIndex]);
  let index = startIndex + 1;

  while (index < lines.length) {
    const rawLine = lines[index];
    const trimmed = rawLine.trim();
    if (trimmed.length === 0) {
      index += 1;
      continue;
    }

    const indent = leadingSpaces(rawLine);
    if (indent <= functionsIndent) {
      break;
    }

    if (indent === functionsIndent + 2 && trimmed.endsWith(":")) {
      const fnName = trimmed.slice(0, -1).trim();
      const fn: FunctionDraft = { name: fnName, triggers: [] };
      index += 1;

      while (index < lines.length) {
        const innerLine = lines[index];
        const innerTrimmed = innerLine.trim();
        if (innerTrimmed.length === 0) {
          index += 1;
          continue;
        }
        const innerIndent = leadingSpaces(innerLine);
        if (innerIndent <= indent) {
          break;
        }

        if (innerTrimmed.startsWith("handler:")) {
          fn.entry = entryFromHandler(innerTrimmed.slice("handler:".length));
          index += 1;
          continue;
        }

        if (innerTrimmed.startsWith("events:")) {
          const eventsIndent = innerIndent;
          index += 1;
          while (index < lines.length) {
            const eventLine = lines[index];
            const eventTrimmed = eventLine.trim();
            if (eventTrimmed.length === 0) {
              index += 1;
              continue;
            }

            const eventIndent = leadingSpaces(eventLine);
            if (eventIndent <= eventsIndent) {
              break;
            }

            if (eventTrimmed.startsWith("- ")) {
              const trigger = parseEventTrigger(lines, index, eventIndent);
              if (trigger) {
                fn.triggers.push(trigger);
              }
            }
            index += 1;
          }
          continue;
        }

        index += 1;
      }

      output.push(fn);
      continue;
    }

    index += 1;
  }

  return output;
}

function toScalar(value: unknown): string {
  if (typeof value === "string") {
    if (/^[A-Za-z0-9_./:@-]+$/.test(value) && !value.includes("://")) {
      return value;
    }
    return JSON.stringify(value);
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (value === null) {
    return "null";
  }
  return JSON.stringify(value);
}

function toYaml(value: unknown, indent = 0): string[] {
  const pad = " ".repeat(indent);
  if (Array.isArray(value)) {
    if (value.length === 0) {
      return [`${pad}[]`];
    }
    const lines: string[] = [];
    for (const item of value) {
      if (item && typeof item === "object" && !Array.isArray(item)) {
        lines.push(`${pad}-`);
        lines.push(...toYaml(item, indent + 2));
      } else if (Array.isArray(item)) {
        lines.push(`${pad}-`);
        lines.push(...toYaml(item, indent + 2));
      } else {
        lines.push(`${pad}- ${toScalar(item)}`);
      }
    }
    return lines;
  }

  if (value && typeof value === "object") {
    const lines: string[] = [];
    for (const [key, entry] of Object.entries(value as Record<string, unknown>)) {
      if (entry === undefined) {
        continue;
      }
      if (entry && typeof entry === "object") {
        lines.push(`${pad}${key}:`);
        lines.push(...toYaml(entry, indent + 2));
      } else {
        lines.push(`${pad}${key}: ${toScalar(entry)}`);
      }
    }
    return lines;
  }

  return [`${pad}${toScalar(value)}`];
}

function migrateServerlessToRunfabric(
  sourceContent: string,
  providerOverride?: string
): MigrationDraft {
  const lines = sourceContent.split(/\r?\n/);
  const warnings: string[] = [];

  const serviceLine = lines.find((line) => line.trim().startsWith("service:"));
  const service = serviceLine ? trimQuotes(serviceLine.split(":").slice(1).join(":")).trim() : "runfabric-service";

  let providerName = "aws-lambda";
  let providerRuntime: string | undefined;
  let providerRegion: string | undefined;
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    if (line.trim() !== "provider:") {
      continue;
    }
    const blockIndent = leadingSpaces(line);
    for (let inner = index + 1; inner < lines.length; inner += 1) {
      const innerLine = lines[inner];
      if (innerLine.trim().length === 0) {
        continue;
      }
      const innerIndent = leadingSpaces(innerLine);
      if (innerIndent <= blockIndent) {
        break;
      }
      const trimmed = innerLine.trim();
      if (trimmed.startsWith("name:")) {
        providerName = parseProviderName(trimmed.slice("name:".length));
      } else if (trimmed.startsWith("runtime:")) {
        providerRuntime = trimQuotes(trimmed.slice("runtime:".length));
      } else if (trimmed.startsWith("region:")) {
        providerRegion = trimQuotes(trimmed.slice("region:".length));
      }
    }
    break;
  }

  if (providerOverride && providerOverride.trim().length > 0) {
    providerName = providerOverride.trim();
  }

  const functionsIndex = lines.findIndex((line) => line.trim() === "functions:");
  const functions = functionsIndex >= 0 ? parseFunctions(lines, functionsIndex) : [];

  const entry = functions.find((fn) => Boolean(fn.entry))?.entry || "src/index.ts";
  const topLevelTriggers: TriggerDraft[] = [];
  for (const fn of functions) {
    for (const trigger of fn.triggers) {
      topLevelTriggers.push(trigger);
    }
  }
  if (topLevelTriggers.length === 0) {
    warnings.push("no function events detected; defaulting to GET /hello trigger");
    topLevelTriggers.push({
      type: "http",
      method: "GET",
      path: "/hello"
    });
  }

  const extensions: Record<string, unknown> = {};
  if (providerName === "aws-lambda" && providerRegion) {
    extensions["aws-lambda"] = { region: providerRegion };
  }

  return {
    service,
    runtime: normalizeRuntime(providerRuntime, warnings),
    entry,
    provider: providerName,
    functions,
    topLevelTriggers,
    warnings,
    extensions: Object.keys(extensions).length > 0 ? extensions : undefined
  };
}

function buildRunfabricYaml(draft: MigrationDraft): string {
  const functions = draft.functions.map((fn) => {
    const out: Record<string, unknown> = { name: fn.name };
    if (fn.entry) {
      out.entry = fn.entry;
    }
    if (fn.triggers.length > 0) {
      out.triggers = fn.triggers;
    }
    return out;
  });

  const output: Record<string, unknown> = {
    service: draft.service,
    runtime: draft.runtime,
    entry: draft.entry,
    providers: [draft.provider],
    triggers: draft.topLevelTriggers
  };

  if (functions.length > 0) {
    output.functions = functions;
  }
  if (draft.extensions) {
    output.extensions = draft.extensions;
  }

  return `${toYaml(output).join("\n")}\n`;
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
    .action(
      async (options: {
        input: string;
        output?: string;
        provider?: string;
        dryRun?: boolean;
        force?: boolean;
        json?: boolean;
      }) => {
        try {
          const inputPath = resolve(process.cwd(), options.input);
          const source = await readFile(inputPath, "utf8");
          const migrated = migrateServerlessToRunfabric(source, options.provider);
          const outputPath =
            options.output && options.output.trim().length > 0
              ? resolve(process.cwd(), options.output)
              : resolve(dirname(inputPath), "runfabric.yml");
          const yaml = buildRunfabricYaml(migrated);

          if (options.dryRun) {
            info(yaml.trimEnd());
          } else {
            if (!options.force) {
              try {
                await access(outputPath, constants.F_OK);
                throw new Error(`output file already exists: ${outputPath}. use --force to overwrite`);
              } catch (accessError) {
                if ((accessError as NodeJS.ErrnoException).code !== "ENOENT") {
                  throw accessError;
                }
              }
            }
            await writeFile(outputPath, yaml, "utf8");
          }

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
        } catch (migrationError) {
          const message = migrationError instanceof Error ? migrationError.message : String(migrationError);
          error(message);
          process.exitCode = 1;
        }
      }
    );
};
