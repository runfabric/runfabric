import { normalizeRuntimeFamily, runtimeFamilyList, type RuntimeFamily } from "@runfabric/core";

export type TriggerDraft =
  | { type: "http"; method: string; path: string }
  | { type: "cron"; schedule: string }
  | { type: "queue"; queue: string }
  | { type: "storage"; bucket: string; events: string[] };

interface FunctionDraft {
  name: string;
  entry?: string;
  triggers: TriggerDraft[];
}

export interface MigrationDraft {
  service: string;
  runtime: RuntimeFamily;
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

const PROVIDER_NAME_ALIASES: Record<string, string> = {
  aws: "aws-lambda",
  "aws-lambda": "aws-lambda",
  google: "gcp-functions",
  gcp: "gcp-functions",
  "gcp-functions": "gcp-functions",
  azure: "azure-functions",
  "azure-functions": "azure-functions",
  cloudflare: "cloudflare-workers",
  "cloudflare-workers": "cloudflare-workers",
  vercel: "vercel",
  netlify: "netlify",
  alibaba: "alibaba-fc",
  "alibaba-fc": "alibaba-fc",
  digitalocean: "digitalocean-functions",
  "digitalocean-functions": "digitalocean-functions",
  fly: "fly-machines",
  "fly-machines": "fly-machines",
  ibm: "ibm-openwhisk",
  openwhisk: "ibm-openwhisk",
  "ibm-openwhisk": "ibm-openwhisk"
};

function parseProviderName(value: string): string {
  return PROVIDER_NAME_ALIASES[value.trim().toLowerCase()] || "aws-lambda";
}

function normalizeRuntime(runtime: string | undefined, warnings: string[]): RuntimeFamily {
  if (!runtime) {
    return "nodejs";
  }

  const normalized = normalizeRuntimeFamily(runtime);
  if (normalized) {
    return normalized;
  }

  warnings.push(
    `runtime ${runtime} is not supported by runfabric (${runtimeFamilyList()}); migrated runtime set to nodejs`
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

  const methodPathMatch = value.match(/^([a-zA-Z]+)\s+(.+)$/);
  if (methodPathMatch) {
    return {
      type: "http",
      method: methodPathMatch[1].toUpperCase(),
      path: normalizeHttpPath(methodPathMatch[2])
    };
  }

  return { type: "http", method: "GET", path: normalizeHttpPath(value) };
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

function parseHttpEventTrigger(
  line: string,
  lines: string[],
  start: number,
  baseIndent: number
): TriggerDraft | undefined {
  if (!line.startsWith("- http:")) {
    return undefined;
  }
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

function parseScheduleEventTrigger(
  line: string,
  lines: string[],
  start: number,
  baseIndent: number
): TriggerDraft | undefined {
  if (!line.startsWith("- schedule:")) {
    return undefined;
  }
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

function parseQueueEventTrigger(
  line: string,
  lines: string[],
  start: number,
  baseIndent: number
): TriggerDraft | undefined {
  if (!line.startsWith("- sqs:")) {
    return undefined;
  }
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

function parseStorageEventTrigger(
  line: string,
  lines: string[],
  start: number,
  baseIndent: number
): TriggerDraft | undefined {
  if (!line.startsWith("- s3:")) {
    return undefined;
  }
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
  if (!bucket) {
    return undefined;
  }
  return { type: "storage", bucket, events: events.length > 0 ? events : ["s3:ObjectCreated:*"] };
}

function parseEventTrigger(lines: string[], start: number, baseIndent: number): TriggerDraft | undefined {
  const line = lines[start].trim();
  if (line.startsWith("- httpApi:")) {
    return parseHttpTrigger(line.slice("- httpApi:".length));
  }
  return (
    parseHttpEventTrigger(line, lines, start, baseIndent) ||
    parseScheduleEventTrigger(line, lines, start, baseIndent) ||
    parseQueueEventTrigger(line, lines, start, baseIndent) ||
    parseStorageEventTrigger(line, lines, start, baseIndent)
  );
}

function parseFunctionEvents(
  lines: string[],
  startIndex: number,
  eventsIndent: number,
  fn: FunctionDraft
): number {
  let index = startIndex;
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
  return index;
}

function parseSingleFunction(lines: string[], startIndex: number, indent: number): [FunctionDraft, number] {
  const name = lines[startIndex].trim().slice(0, -1).trim();
  const fn: FunctionDraft = { name, triggers: [] };
  let index = startIndex + 1;

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
      index = parseFunctionEvents(lines, index + 1, innerIndent, fn);
      continue;
    }
    index += 1;
  }

  return [fn, index];
}

function parseFunctions(lines: string[], startIndex: number): FunctionDraft[] {
  const output: FunctionDraft[] = [];
  const functionsIndent = leadingSpaces(lines[startIndex]);
  let index = startIndex + 1;

  while (index < lines.length) {
    const rawLine = lines[index];
    const trimmed = rawLine.trim();
    if (!trimmed) {
      index += 1;
      continue;
    }
    const indent = leadingSpaces(rawLine);
    if (indent <= functionsIndent) {
      break;
    }
    if (indent === functionsIndent + 2 && trimmed.endsWith(":")) {
      const [fn, nextIndex] = parseSingleFunction(lines, index, indent);
      output.push(fn);
      index = nextIndex;
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

function readServiceName(lines: string[]): string {
  const serviceLine = lines.find((line) => line.trim().startsWith("service:"));
  if (!serviceLine) {
    return "runfabric-service";
  }
  return trimQuotes(serviceLine.split(":").slice(1).join(":")).trim() || "runfabric-service";
}

function readProviderBlock(lines: string[]): {
  providerName: string;
  providerRuntime?: string;
  providerRegion?: string;
} {
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

  return { providerName, providerRuntime, providerRegion };
}

function collectTopLevelTriggers(functions: FunctionDraft[], warnings: string[]): TriggerDraft[] {
  const topLevelTriggers = functions.flatMap((fn) => fn.triggers);
  if (topLevelTriggers.length > 0) {
    return topLevelTriggers;
  }
  warnings.push("no function events detected; defaulting to GET /hello trigger");
  return [{ type: "http", method: "GET", path: "/hello" }];
}

export function migrateServerlessToRunfabric(
  sourceContent: string,
  providerOverride?: string
): MigrationDraft {
  const lines = sourceContent.split(/\r?\n/);
  const warnings: string[] = [];
  const service = readServiceName(lines);
  const providerBlock = readProviderBlock(lines);
  const providerName =
    providerOverride && providerOverride.trim().length > 0
      ? providerOverride.trim()
      : providerBlock.providerName;
  const functionsIndex = lines.findIndex((line) => line.trim() === "functions:");
  const functions = functionsIndex >= 0 ? parseFunctions(lines, functionsIndex) : [];
  const entry = functions.find((fn) => Boolean(fn.entry))?.entry || "src/index.ts";
  const topLevelTriggers = collectTopLevelTriggers(functions, warnings);
  const extensions =
    providerName === "aws-lambda" && providerBlock.providerRegion
      ? { "aws-lambda": { region: providerBlock.providerRegion } }
      : undefined;

  return {
    service,
    runtime: normalizeRuntime(providerBlock.providerRuntime, warnings),
    entry,
    provider: providerName,
    functions,
    topLevelTriggers,
    warnings,
    extensions
  };
}

export function buildRunfabricYaml(draft: MigrationDraft): string {
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
