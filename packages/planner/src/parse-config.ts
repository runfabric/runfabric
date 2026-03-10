import type { FunctionConfig, ProjectConfig, TriggerConfig } from "@runfabric/core";

interface ParsedLine {
  indent: number;
  content: string;
  line: number;
}

interface YamlKeyValue {
  key: string;
  hasValue: boolean;
  value: string;
}

export interface ParseProjectConfigOptions {
  stage?: string;
}

interface StageOverride {
  runtime?: string;
  entry?: string;
  providers?: string[];
  triggers?: TriggerConfig[];
  functions?: FunctionConfig[];
  hooks?: string[];
  resources?: ProjectConfig["resources"];
  env?: Record<string, string>;
  params?: Record<string, string>;
  extensions?: Record<string, Record<string, unknown>>;
}

type ExtensionValueType = "string" | "number" | "boolean";

const providerExtensionSchema: Record<string, Record<string, ExtensionValueType>> = {
  "aws-lambda": {
    stage: "string",
    region: "string"
  },
  "gcp-functions": {
    region: "string"
  },
  "azure-functions": {
    functionApp: "string",
    routePrefix: "string"
  },
  "cloudflare-workers": {
    scriptName: "string"
  },
  vercel: {
    projectName: "string"
  },
  netlify: {
    siteName: "string"
  },
  "alibaba-fc": {
    region: "string"
  },
  "digitalocean-functions": {
    namespace: "string",
    region: "string"
  },
  "fly-machines": {
    appName: "string",
    region: "string"
  },
  "ibm-openwhisk": {
    namespace: "string"
  }
};

function unquote(value: string): string {
  const trimmed = value.trim();
  if (
    (trimmed.startsWith("\"") && trimmed.endsWith("\"")) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

function parseScalar(value: string): unknown {
  const trimmed = value.trim();
  if (trimmed === "true") {
    return true;
  }
  if (trimmed === "false") {
    return false;
  }
  if (trimmed === "null" || trimmed === "~") {
    return null;
  }
  const numeric = Number(trimmed);
  if (!Number.isNaN(numeric) && trimmed !== "") {
    return numeric;
  }
  return unquote(trimmed);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function parseInlineKeyValue(input: string): YamlKeyValue | null {
  const separator = input.indexOf(":");
  if (separator <= 0) {
    return null;
  }

  const key = input.slice(0, separator).trim();
  if (!key) {
    return null;
  }

  const valuePart = input.slice(separator + 1);
  const hasValue = valuePart.trim().length > 0;
  return {
    key,
    hasValue,
    value: valuePart.trim()
  };
}

function prepareLines(content: string): ParsedLine[] {
  const parsed: ParsedLine[] = [];
  const rawLines = content.split(/\r?\n/);

  for (let index = 0; index < rawLines.length; index += 1) {
    const rawLine = rawLines[index];
    const trimmed = rawLine.trim();
    if (!trimmed || trimmed.startsWith("#")) {
      continue;
    }

    const indent = rawLine.match(/^ */)?.[0].length || 0;
    if (indent % 2 !== 0) {
      throw new Error(`Invalid runfabric.yml: line ${index + 1} uses non-2-space indentation`);
    }

    parsed.push({
      indent,
      content: trimmed,
      line: index + 1
    });
  }

  return parsed;
}

function parseObject(
  lines: ParsedLine[],
  state: { index: number },
  indent: number
): Record<string, unknown> {
  const object: Record<string, unknown> = {};

  while (state.index < lines.length) {
    const line = lines[state.index];
    if (line.indent < indent) {
      break;
    }
    if (line.indent > indent) {
      throw new Error(`Invalid runfabric.yml: unexpected indentation at line ${line.line}`);
    }
    if (line.content.startsWith("-")) {
      break;
    }

    const keyValue = parseInlineKeyValue(line.content);
    if (!keyValue) {
      throw new Error(`Invalid runfabric.yml: expected key/value at line ${line.line}`);
    }

    if (keyValue.hasValue) {
      object[keyValue.key] = parseScalar(keyValue.value);
      state.index += 1;
      continue;
    }

    state.index += 1;
    if (state.index >= lines.length || lines[state.index].indent <= indent) {
      object[keyValue.key] = {};
      continue;
    }

    object[keyValue.key] = parseNode(lines, state, lines[state.index].indent);
  }

  return object;
}

function parseArray(lines: ParsedLine[], state: { index: number }, indent: number): unknown[] {
  const values: unknown[] = [];

  while (state.index < lines.length) {
    const line = lines[state.index];
    if (line.indent < indent) {
      break;
    }
    if (line.indent > indent) {
      throw new Error(`Invalid runfabric.yml: unexpected indentation at line ${line.line}`);
    }
    if (!line.content.startsWith("-")) {
      break;
    }

    const rest = line.content.slice(1).trimStart();
    if (!rest) {
      state.index += 1;
      if (state.index >= lines.length || lines[state.index].indent <= indent) {
        values.push({});
      } else {
        values.push(parseNode(lines, state, lines[state.index].indent));
      }
      continue;
    }

    const inlineKeyValue = parseInlineKeyValue(rest);
    if (!inlineKeyValue) {
      values.push(parseScalar(rest));
      state.index += 1;
      continue;
    }

    const item: Record<string, unknown> = {};
    if (inlineKeyValue.hasValue) {
      item[inlineKeyValue.key] = parseScalar(inlineKeyValue.value);
      state.index += 1;
    } else {
      state.index += 1;
      if (state.index >= lines.length || lines[state.index].indent <= indent) {
        item[inlineKeyValue.key] = {};
      } else {
        item[inlineKeyValue.key] = parseNode(lines, state, lines[state.index].indent);
      }
    }

    if (state.index < lines.length && lines[state.index].indent > indent) {
      const continuation = parseObject(lines, state, indent + 2);
      for (const [key, value] of Object.entries(continuation)) {
        item[key] = value;
      }
    }

    values.push(item);
  }

  return values;
}

function parseNode(lines: ParsedLine[], state: { index: number }, indent: number): unknown {
  const line = lines[state.index];
  if (!line) {
    return {};
  }

  if (line.indent !== indent) {
    throw new Error(`Invalid runfabric.yml: invalid indentation near line ${line.line}`);
  }

  if (line.content.startsWith("-")) {
    return parseArray(lines, state, indent);
  }
  return parseObject(lines, state, indent);
}

function parseYamlDocument(content: string): unknown {
  const lines = prepareLines(content);
  if (lines.length === 0) {
    return {};
  }

  const state = { index: 0 };
  const root = parseNode(lines, state, lines[0].indent);
  if (state.index !== lines.length) {
    const unparsed = lines[state.index];
    throw new Error(`Invalid runfabric.yml: could not parse line ${unparsed.line}`);
  }
  return root;
}

function readRequiredString(
  source: Record<string, unknown>,
  key: string,
  errors: string[]
): string {
  const value = source[key];
  if (typeof value !== "string" || value.trim().length === 0) {
    errors.push(`${key} must be a non-empty string`);
    return "";
  }
  return value.trim();
}

function readOptionalString(
  source: Record<string, unknown>,
  key: string,
  errors: string[]
): string | undefined {
  if (!(key in source)) {
    return undefined;
  }
  const value = source[key];
  if (typeof value !== "string" || value.trim().length === 0) {
    errors.push(`${key} must be a non-empty string`);
    return undefined;
  }
  return value.trim();
}

function readStringArrayAtPath(
  value: unknown,
  path: string,
  errors: string[],
  minSize = 0
): string[] {
  if (!Array.isArray(value)) {
    errors.push(`${path} must be an array`);
    return [];
  }

  const values: string[] = [];
  value.forEach((item, index) => {
    if (typeof item !== "string" || item.trim().length === 0) {
      errors.push(`${path}[${index}] must be a non-empty string`);
      return;
    }
    values.push(item.trim());
  });

  if (values.length < minSize) {
    errors.push(`${path} must contain at least ${minSize} value(s)`);
  }

  return values;
}

function readStringArray(
  source: Record<string, unknown>,
  key: string,
  errors: string[],
  minSize = 0
): string[] {
  return readStringArrayAtPath(source[key], key, errors, minSize);
}

function readTriggerArrayAtPath(
  value: unknown,
  path: string,
  errors: string[],
  minSize = 1
): TriggerConfig[] {
  if (!Array.isArray(value)) {
    errors.push(`${path} must be an array`);
    return [];
  }

  const triggers: TriggerConfig[] = [];
  value.forEach((item, index) => {
    if (!isRecord(item)) {
      errors.push(`${path}[${index}] must be an object`);
      return;
    }

    const trigger: TriggerConfig = { type: "http" };
    for (const [field, rawValue] of Object.entries(item)) {
      if (typeof rawValue === "string") {
        trigger[field] = rawValue;
      } else if (typeof rawValue === "number" || typeof rawValue === "boolean") {
        trigger[field] = String(rawValue);
      } else {
        errors.push(`${path}[${index}].${field} must be a scalar value`);
      }
    }

    if (!trigger.type || trigger.type.trim().length === 0) {
      errors.push(`${path}[${index}].type must be a non-empty string`);
      return;
    }

    trigger.type = trigger.type.trim();
    triggers.push(trigger);
  });

  if (triggers.length < minSize) {
    errors.push(`${path} must contain at least ${minSize} trigger(s)`);
  }

  return triggers;
}

function readTriggerArray(source: Record<string, unknown>, errors: string[]): TriggerConfig[] {
  return readTriggerArrayAtPath(source.triggers, "triggers", errors, 1);
}

function readFunctionsAtPath(
  value: unknown,
  path: string,
  errors: string[]
): FunctionConfig[] | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!Array.isArray(value)) {
    errors.push(`${path} must be an array`);
    return undefined;
  }

  const functions: FunctionConfig[] = [];
  value.forEach((item, index) => {
    if (!isRecord(item)) {
      errors.push(`${path}[${index}] must be an object`);
      return;
    }

    const name = readRequiredString(item, "name", errors);
    const entry = readOptionalString(item, "entry", errors);
    const runtime = readOptionalString(item, "runtime", errors);
    const triggers = "triggers" in item
      ? readTriggerArrayAtPath(item.triggers, `${path}[${index}].triggers`, errors, 1)
      : undefined;
    const resources = readResourcesAtPath(item.resources, `${path}[${index}].resources`, errors);

    if (!name) {
      return;
    }

    functions.push({
      name,
      entry,
      runtime,
      triggers,
      resources
    });
  });

  return functions.length > 0 ? functions : undefined;
}

function readFunctions(
  source: Record<string, unknown>,
  errors: string[]
): FunctionConfig[] | undefined {
  return readFunctionsAtPath(source.functions, "functions", errors);
}

function readStringRecordAtPath(
  value: unknown,
  path: string,
  errors: string[]
): Record<string, string> | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path} must be an object`);
    return undefined;
  }

  const out: Record<string, string> = {};
  for (const [entryKey, entryValue] of Object.entries(value)) {
    if (typeof entryValue !== "string") {
      errors.push(`${path}.${entryKey} must be a string`);
      continue;
    }
    out[entryKey] = entryValue;
  }
  return out;
}

function readStringRecord(
  source: Record<string, unknown>,
  key: string,
  errors: string[]
): Record<string, string> | undefined {
  return readStringRecordAtPath(source[key], key, errors);
}

function readResourcesAtPath(
  value: unknown,
  path: string,
  errors: string[]
): ProjectConfig["resources"] | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path} must be an object`);
    return undefined;
  }

  const resources: ProjectConfig["resources"] = {};
  if ("memory" in value) {
    if (typeof value.memory !== "number" || Number.isNaN(value.memory)) {
      errors.push(`${path}.memory must be a number`);
    } else {
      resources.memory = value.memory;
    }
  }
  if ("timeout" in value) {
    if (typeof value.timeout !== "number" || Number.isNaN(value.timeout)) {
      errors.push(`${path}.timeout must be a number`);
    } else {
      resources.timeout = value.timeout;
    }
  }

  if (resources.memory === undefined && resources.timeout === undefined) {
    return undefined;
  }
  return resources;
}

function readExtensionsAtPath(
  value: unknown,
  path: string,
  errors: string[]
): Record<string, Record<string, unknown>> | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path} must be an object`);
    return undefined;
  }

  const out: Record<string, Record<string, unknown>> = {};
  for (const [provider, providerConfig] of Object.entries(value)) {
    if (!isRecord(providerConfig)) {
      errors.push(`${path}.${provider} must be an object`);
      continue;
    }

    const extensionValues: Record<string, unknown> = {};
    for (const [field, fieldValue] of Object.entries(providerConfig)) {
      if (
        typeof fieldValue === "string" ||
        typeof fieldValue === "number" ||
        typeof fieldValue === "boolean"
      ) {
        extensionValues[field] = fieldValue;
      } else {
        errors.push(`${path}.${provider}.${field} must be a scalar value`);
      }
    }
    out[provider] = extensionValues;
  }

  return out;
}

function readExtensions(
  source: Record<string, unknown>,
  errors: string[]
): Record<string, Record<string, unknown>> | undefined {
  return readExtensionsAtPath(source.extensions, "extensions", errors);
}

function mergeExtensions(
  base: Record<string, Record<string, unknown>> | undefined,
  override: Record<string, Record<string, unknown>> | undefined
): Record<string, Record<string, unknown>> | undefined {
  if (!base && !override) {
    return undefined;
  }

  const merged: Record<string, Record<string, unknown>> = {
    ...(base || {})
  };

  if (override) {
    for (const [provider, values] of Object.entries(override)) {
      merged[provider] = {
        ...(merged[provider] || {}),
        ...values
      };
    }
  }

  return merged;
}

function applyStageOverride(base: ProjectConfig, override: StageOverride): ProjectConfig {
  const mergedResources =
    override.resources || base.resources
      ? {
          ...(base.resources || {}),
          ...(override.resources || {})
        }
      : undefined;

  return {
    ...base,
    runtime: override.runtime || base.runtime,
    entry: override.entry || base.entry,
    providers: override.providers || base.providers,
    triggers: override.triggers || base.triggers,
    functions: override.functions || base.functions,
    hooks: override.hooks || base.hooks,
    resources: mergedResources,
    env: {
      ...(base.env || {}),
      ...(override.env || {})
    },
    params: {
      ...(base.params || {}),
      ...(override.params || {})
    },
    extensions: mergeExtensions(base.extensions, override.extensions)
  };
}

function readStageOverride(
  source: Record<string, unknown>,
  path: string,
  errors: string[]
): StageOverride {
  const override: StageOverride = {};

  const runtime = readOptionalString(source, "runtime", errors);
  if (runtime) {
    override.runtime = runtime;
  }

  const entry = readOptionalString(source, "entry", errors);
  if (entry) {
    override.entry = entry;
  }

  if ("providers" in source) {
    override.providers = readStringArrayAtPath(source.providers, `${path}.providers`, errors, 1);
  }

  if ("triggers" in source) {
    override.triggers = readTriggerArrayAtPath(source.triggers, `${path}.triggers`, errors, 1);
  }
  if ("functions" in source) {
    override.functions = readFunctionsAtPath(source.functions, `${path}.functions`, errors);
  }
  if ("hooks" in source) {
    override.hooks = readStringArrayAtPath(source.hooks, `${path}.hooks`, errors);
  }

  override.resources = readResourcesAtPath(source.resources, `${path}.resources`, errors);
  override.env = readStringRecordAtPath(source.env, `${path}.env`, errors);
  override.params = readStringRecordAtPath(source.params, `${path}.params`, errors);
  override.extensions = readExtensionsAtPath(source.extensions, `${path}.extensions`, errors);

  return override;
}

function readStageOverrides(
  source: Record<string, unknown>,
  errors: string[]
): Record<string, StageOverride> | undefined {
  if (!("stages" in source)) {
    return undefined;
  }

  const value = source.stages;
  if (!isRecord(value)) {
    errors.push("stages must be an object");
    return undefined;
  }

  const overrides: Record<string, StageOverride> = {};
  for (const [stageName, stageConfig] of Object.entries(value)) {
    if (!isRecord(stageConfig)) {
      errors.push(`stages.${stageName} must be an object`);
      continue;
    }
    overrides[stageName] = readStageOverride(stageConfig, `stages.${stageName}`, errors);
  }

  return overrides;
}

function validateExtensionTypes(
  extensions: Record<string, Record<string, unknown>> | undefined,
  providers: string[],
  errors: string[]
): void {
  if (!extensions) {
    return;
  }

  for (const [provider, values] of Object.entries(extensions)) {
    if (!providers.includes(provider)) {
      errors.push(`extensions.${provider} is set but provider is not listed in providers`);
      continue;
    }

    const schema = providerExtensionSchema[provider];
    if (!schema) {
      continue;
    }

    for (const [field, value] of Object.entries(values)) {
      const expectedType = schema[field];
      if (!expectedType) {
        errors.push(`extensions.${provider}.${field} is not a supported field`);
        continue;
      }

      if (typeof value !== expectedType) {
        errors.push(
          `extensions.${provider}.${field} must be ${expectedType}, received ${typeof value}`
        );
      }
    }
  }
}

function validateProjectConfigShape(
  raw: unknown,
  options: ParseProjectConfigOptions = {}
): ProjectConfig {
  if (!isRecord(raw)) {
    throw new Error("Invalid runfabric.yml: root document must be an object");
  }

  const errors: string[] = [];
  const service = readRequiredString(raw, "service", errors);
  const runtime = readRequiredString(raw, "runtime", errors);
  const entry = readRequiredString(raw, "entry", errors);
  const providers = readStringArray(raw, "providers", errors, 1);
  const triggers = readTriggerArray(raw, errors);
  const functions = readFunctions(raw, errors);
  const hooks = "hooks" in raw ? readStringArrayAtPath(raw.hooks, "hooks", errors) : undefined;
  const resources = readResourcesAtPath(raw.resources, "resources", errors);
  const env = readStringRecord(raw, "env", errors);
  const params = readStringRecord(raw, "params", errors);
  const extensions = readExtensions(raw, errors);
  const stageOverrides = readStageOverrides(raw, errors);

  const selectedStage = (
    options.stage ||
    process.env.RUNFABRIC_STAGE ||
    "default"
  ).trim() || "default";

  let project: ProjectConfig = {
    service,
    runtime,
    entry,
    providers,
    triggers,
    functions,
    hooks,
    resources,
    env,
    params,
    extensions,
    stage: selectedStage
  };

  if (stageOverrides) {
    if (stageOverrides.default) {
      project = applyStageOverride(project, stageOverrides.default);
    }

    if (selectedStage !== "default") {
      const selectedOverride = stageOverrides[selectedStage];
      if (!selectedOverride) {
        errors.push(`stages.${selectedStage} is not defined`);
      } else {
        project = applyStageOverride(project, selectedOverride);
      }
    }
  }

  validateExtensionTypes(project.extensions, project.providers, errors);

  if (errors.length > 0) {
    throw new Error(`Invalid runfabric.yml:\n- ${errors.join("\n- ")}`);
  }

  return project;
}

export function parseProjectConfig(
  content: string,
  options: ParseProjectConfigOptions = {}
): ProjectConfig {
  const parsed = parseYaml(content);
  return validateProjectConfigShape(parsed, options);
}

export function parseYaml(content: string): unknown {
  return parseYamlDocument(content);
}
