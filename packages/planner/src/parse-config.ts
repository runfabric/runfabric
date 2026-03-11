import {
  AwsIamEffectEnum,
  AwsQueueFunctionResponseTypeEnum,
  TriggerEnum
} from "@runfabric/core";
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
  secrets?: Record<string, string>;
  workflows?: ProjectConfig["workflows"];
  params?: Record<string, string>;
  extensions?: ProjectConfig["extensions"];
  state?: ProjectConfig["state"];
}

type ExtensionValueType = "string" | "number" | "boolean" | "object";

const providerExtensionSchema: Record<string, Record<string, ExtensionValueType>> = {
  "aws-lambda": {
    stage: "string",
    region: "string",
    roleArn: "string",
    functionName: "string",
    runtime: "string",
    iam: "object"
  },
  "gcp-functions": {
    region: "string"
  },
  "azure-functions": {
    functionApp: "string",
    functionAppName: "string",
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

function resolveEnvBindingInString(
  value: string,
  path: string,
  errors: string[]
): string {
  const envPattern = /\$\{env:([A-Za-z_][A-Za-z0-9_]*)(?:,\s*([^}]*))?\}/g;

  return value.replace(envPattern, (_match, envNameRaw, defaultValueRaw) => {
    const envName = String(envNameRaw || "").trim();
    const envValue = process.env[envName];
    if (envValue !== undefined) {
      return envValue;
    }

    if (typeof defaultValueRaw === "string") {
      return unquote(defaultValueRaw.trim());
    }

    errors.push(`${path} references missing environment variable ${envName}`);
    return "";
  });
}

function resolveDynamicBindingsAtPath(
  value: unknown,
  path: string,
  errors: string[]
): unknown {
  if (typeof value === "string") {
    return resolveEnvBindingInString(value, path, errors);
  }

  if (Array.isArray(value)) {
    return value.map((item, index) =>
      resolveDynamicBindingsAtPath(item, `${path}[${index}]`, errors)
    );
  }

  if (isRecord(value)) {
    const out: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value)) {
      out[key] = resolveDynamicBindingsAtPath(item, `${path}.${key}`, errors);
    }
    return out;
  }

  return value;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isScalar(value: unknown): value is string | number | boolean | null {
  return (
    typeof value === "string" ||
    typeof value === "number" ||
    typeof value === "boolean" ||
    value === null
  );
}

function isStringArray(value: unknown): value is string[] {
  return (
    Array.isArray(value) &&
    value.every((entry) => typeof entry === "string" && entry.trim().length > 0)
  );
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function parseInlineKeyValue(input: string): YamlKeyValue | null {
  let separator = -1;
  for (let index = 0; index < input.length; index += 1) {
    if (input[index] !== ":") {
      continue;
    }
    const next = input[index + 1];
    if (next === undefined || /\s/.test(next)) {
      separator = index;
      break;
    }
  }
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

function validateQueueTriggerAtPath(trigger: TriggerConfig, path: string, errors: string[]): void {
  if (typeof trigger.queue !== "string" || trigger.queue.trim().length === 0) {
    errors.push(`${path}.queue must be a non-empty string`);
  }

  for (const field of ["batchSize", "maximumBatchingWindowSeconds", "maximumConcurrency"] as const) {
    const value = trigger[field];
    if (value === undefined) {
      continue;
    }
    if (!isFiniteNumber(value)) {
      errors.push(`${path}.${field} must be a number`);
      continue;
    }
    if (value < 0) {
      errors.push(`${path}.${field} must be >= 0`);
    }
  }

  if (trigger.enabled !== undefined && typeof trigger.enabled !== "boolean") {
    errors.push(`${path}.enabled must be a boolean`);
  }

  if (trigger.functionResponseType !== undefined) {
    if (typeof trigger.functionResponseType !== "string") {
      errors.push(`${path}.functionResponseType must be a string`);
    } else if (
      trigger.functionResponseType !== AwsQueueFunctionResponseTypeEnum.ReportBatchItemFailures
    ) {
      errors.push(`${path}.functionResponseType must be ReportBatchItemFailures`);
    } else {
      trigger.functionResponseType = AwsQueueFunctionResponseTypeEnum.ReportBatchItemFailures;
    }
  }
}

function validateStorageTriggerAtPath(trigger: TriggerConfig, path: string, errors: string[]): void {
  if (typeof trigger.bucket !== "string" || trigger.bucket.trim().length === 0) {
    errors.push(`${path}.bucket must be a non-empty string`);
  }

  if (!isStringArray(trigger.events) || trigger.events.length === 0) {
    errors.push(`${path}.events must be an array with at least one event string`);
  }

  if (trigger.prefix !== undefined && (typeof trigger.prefix !== "string" || trigger.prefix.trim().length === 0)) {
    errors.push(`${path}.prefix must be a non-empty string`);
  }

  if (trigger.suffix !== undefined && (typeof trigger.suffix !== "string" || trigger.suffix.trim().length === 0)) {
    errors.push(`${path}.suffix must be a non-empty string`);
  }

  if (trigger.existingBucket !== undefined && typeof trigger.existingBucket !== "boolean") {
    errors.push(`${path}.existingBucket must be a boolean`);
  }
}

function validateEventBridgeTriggerAtPath(
  trigger: TriggerConfig,
  path: string,
  errors: string[]
): void {
  if (trigger.bus !== undefined && (typeof trigger.bus !== "string" || trigger.bus.trim().length === 0)) {
    errors.push(`${path}.bus must be a non-empty string`);
  }
  if (!isRecord(trigger.pattern)) {
    errors.push(`${path}.pattern must be an object`);
  }
}

function validatePubSubTriggerAtPath(trigger: TriggerConfig, path: string, errors: string[]): void {
  if (typeof trigger.topic !== "string" || trigger.topic.trim().length === 0) {
    errors.push(`${path}.topic must be a non-empty string`);
  }
  if (
    trigger.subscription !== undefined &&
    (typeof trigger.subscription !== "string" || trigger.subscription.trim().length === 0)
  ) {
    errors.push(`${path}.subscription must be a non-empty string`);
  }
}

function validateKafkaTriggerAtPath(trigger: TriggerConfig, path: string, errors: string[]): void {
  if (!isStringArray(trigger.brokers) || trigger.brokers.length === 0) {
    errors.push(`${path}.brokers must be an array with at least one broker string`);
  }
  if (typeof trigger.topic !== "string" || trigger.topic.trim().length === 0) {
    errors.push(`${path}.topic must be a non-empty string`);
  }
  if (typeof trigger.groupId !== "string" || trigger.groupId.trim().length === 0) {
    errors.push(`${path}.groupId must be a non-empty string`);
  }
}

function validateRabbitMqTriggerAtPath(trigger: TriggerConfig, path: string, errors: string[]): void {
  if (typeof trigger.queue !== "string" || trigger.queue.trim().length === 0) {
    errors.push(`${path}.queue must be a non-empty string`);
  }
  if (
    trigger.exchange !== undefined &&
    (typeof trigger.exchange !== "string" || trigger.exchange.trim().length === 0)
  ) {
    errors.push(`${path}.exchange must be a non-empty string`);
  }
  if (
    trigger.routingKey !== undefined &&
    (typeof trigger.routingKey !== "string" || trigger.routingKey.trim().length === 0)
  ) {
    errors.push(`${path}.routingKey must be a non-empty string`);
  }
}

function parseTriggerType(value: string): TriggerEnum | null {
  const normalized = value.trim().toLowerCase();
  if (normalized === TriggerEnum.Http) {
    return TriggerEnum.Http;
  }
  if (normalized === TriggerEnum.Cron) {
    return TriggerEnum.Cron;
  }
  if (normalized === TriggerEnum.Queue) {
    return TriggerEnum.Queue;
  }
  if (normalized === TriggerEnum.Storage) {
    return TriggerEnum.Storage;
  }
  if (normalized === TriggerEnum.EventBridge) {
    return TriggerEnum.EventBridge;
  }
  if (normalized === TriggerEnum.PubSub) {
    return TriggerEnum.PubSub;
  }
  if (normalized === TriggerEnum.Kafka) {
    return TriggerEnum.Kafka;
  }
  if (normalized === TriggerEnum.RabbitMq) {
    return TriggerEnum.RabbitMq;
  }
  return null;
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

    const trigger: TriggerConfig = { type: TriggerEnum.Http };
    for (const [field, rawValue] of Object.entries(item)) {
      if (field === "type") {
        if (typeof rawValue !== "string" || rawValue.trim().length === 0) {
          errors.push(`${path}[${index}].type must be a non-empty string`);
          continue;
        }
        const parsedType = parseTriggerType(rawValue);
        if (!parsedType) {
          errors.push(
            `${path}[${index}].type must be one of: http, cron, queue, storage, eventbridge, pubsub, kafka, rabbitmq`
          );
          continue;
        }
        trigger.type = parsedType;
        continue;
      }

      if (typeof rawValue === "string") {
        trigger[field] = rawValue.trim();
      } else if (typeof rawValue === "number" || typeof rawValue === "boolean") {
        trigger[field] = rawValue;
      } else if (Array.isArray(rawValue)) {
        if (!isStringArray(rawValue)) {
          errors.push(`${path}[${index}].${field} must be an array of non-empty strings`);
          continue;
        }
        trigger[field] = rawValue.map((entry) => entry.trim());
      } else if (isRecord(rawValue)) {
        trigger[field] = rawValue;
      } else {
        errors.push(`${path}[${index}].${field} has an unsupported value`);
      }
    }

    if (trigger.type === TriggerEnum.Queue) {
      validateQueueTriggerAtPath(trigger, `${path}[${index}]`, errors);
    }
    if (trigger.type === TriggerEnum.Storage) {
      validateStorageTriggerAtPath(trigger, `${path}[${index}]`, errors);
    }
    if (trigger.type === TriggerEnum.EventBridge) {
      validateEventBridgeTriggerAtPath(trigger, `${path}[${index}]`, errors);
    }
    if (trigger.type === TriggerEnum.PubSub) {
      validatePubSubTriggerAtPath(trigger, `${path}[${index}]`, errors);
    }
    if (trigger.type === TriggerEnum.Kafka) {
      validateKafkaTriggerAtPath(trigger, `${path}[${index}]`, errors);
    }
    if (trigger.type === TriggerEnum.RabbitMq) {
      validateRabbitMqTriggerAtPath(trigger, `${path}[${index}]`, errors);
    }
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
    const env = readStringRecordAtPath(item.env, `${path}[${index}].env`, errors);

    if (!name) {
      return;
    }

    functions.push({
      name,
      entry,
      runtime,
      triggers,
      resources,
      env
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

function readNamedResourceArrayAtPath(
  value: unknown,
  path: string,
  errors: string[]
): Array<{ name: string; [key: string]: unknown }> | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!Array.isArray(value)) {
    errors.push(`${path} must be an array`);
    return undefined;
  }

  const resources: Array<{ name: string; [key: string]: unknown }> = [];
  value.forEach((item, index) => {
    if (!isRecord(item)) {
      errors.push(`${path}[${index}] must be an object`);
      return;
    }

    if (typeof item.name !== "string" || item.name.trim().length === 0) {
      errors.push(`${path}[${index}].name must be a non-empty string`);
      return;
    }

    const normalized: { name: string; [key: string]: unknown } = {
      name: item.name.trim()
    };

    for (const [key, rawValue] of Object.entries(item)) {
      if (key === "name") {
        continue;
      }
      const cloned = cloneExtensionValueAtPath(rawValue, `${path}[${index}].${key}`, errors);
      if (cloned !== undefined) {
        normalized[key] = cloned;
      }
    }
    resources.push(normalized);
  });

  return resources;
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

  resources.queues = readNamedResourceArrayAtPath(value.queues, `${path}.queues`, errors);
  resources.buckets = readNamedResourceArrayAtPath(value.buckets, `${path}.buckets`, errors);
  resources.topics = readNamedResourceArrayAtPath(value.topics, `${path}.topics`, errors);
  resources.databases = readNamedResourceArrayAtPath(value.databases, `${path}.databases`, errors);

  if (
    resources.memory === undefined &&
    resources.timeout === undefined &&
    resources.queues === undefined &&
    resources.buckets === undefined &&
    resources.topics === undefined &&
    resources.databases === undefined
  ) {
    return undefined;
  }
  return resources;
}

function readWorkflowsAtPath(
  value: unknown,
  path: string,
  errors: string[]
): ProjectConfig["workflows"] | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!Array.isArray(value)) {
    errors.push(`${path} must be an array`);
    return undefined;
  }

  const workflows: NonNullable<ProjectConfig["workflows"]> = [];
  value.forEach((item, index) => {
    if (!isRecord(item)) {
      errors.push(`${path}[${index}] must be an object`);
      return;
    }

    const workflowPath = `${path}[${index}]`;
    const name = readRequiredString(item, "name", errors);
    if (!Array.isArray(item.steps)) {
      errors.push(`${workflowPath}.steps must be an array`);
      return;
    }

    const steps = item.steps
      .map((step, stepIndex) => {
        if (!isRecord(step)) {
          errors.push(`${workflowPath}.steps[${stepIndex}] must be an object`);
          return undefined;
        }

        const stepPath = `${workflowPath}.steps[${stepIndex}]`;
        const fn = readRequiredString(step, "function", errors);
        const next = readOptionalString(step, "next", errors);
        let retry: { attempts?: number; backoffSeconds?: number } | undefined;
        if ("retry" in step) {
          if (!isRecord(step.retry)) {
            errors.push(`${stepPath}.retry must be an object`);
          } else {
            retry = {};
            const attempts = readOptionalNumberAtPath(step.retry.attempts, `${stepPath}.retry.attempts`, errors, 1);
            const backoffSeconds = readOptionalNumberAtPath(
              step.retry.backoffSeconds,
              `${stepPath}.retry.backoffSeconds`,
              errors,
              0
            );
            if (attempts !== undefined) {
              retry.attempts = attempts;
            }
            if (backoffSeconds !== undefined) {
              retry.backoffSeconds = backoffSeconds;
            }
          }
        }
        const timeoutSeconds = readOptionalNumberAtPath(step.timeoutSeconds, `${stepPath}.timeoutSeconds`, errors, 1);

        if (!fn) {
          return undefined;
        }

        const normalizedStep: {
          function: string;
          next?: string;
          retry?: { attempts?: number; backoffSeconds?: number };
          timeoutSeconds?: number;
        } = {
          function: fn
        };
        if (next) {
          normalizedStep.next = next;
        }
        if (retry && Object.keys(retry).length > 0) {
          normalizedStep.retry = retry;
        }
        if (timeoutSeconds !== undefined) {
          normalizedStep.timeoutSeconds = timeoutSeconds;
        }
        return normalizedStep;
      })
      .filter((step): step is NonNullable<ProjectConfig["workflows"]>[number]["steps"][number] => Boolean(step));

    if (!name) {
      return;
    }
    if (steps.length === 0) {
      errors.push(`${workflowPath}.steps must contain at least one step`);
      return;
    }

    workflows.push({
      name,
      steps
    });
  });

  return workflows.length > 0 ? workflows : undefined;
}

function readSecretsAtPath(
  value: unknown,
  path: string,
  errors: string[]
): Record<string, string> | undefined {
  const secrets = readStringRecordAtPath(value, path, errors);
  if (!secrets) {
    return undefined;
  }

  const out: Record<string, string> = {};
  for (const [key, secretValue] of Object.entries(secrets)) {
    if (!secretValue.startsWith("secret://") || secretValue.trim().length <= "secret://".length) {
      errors.push(`${path}.${key} must use secret://<ref> format`);
      continue;
    }
    out[key] = secretValue.trim();
  }

  return Object.keys(out).length > 0 ? out : undefined;
}

const supportedStateBackends = ["local", "postgres", "s3", "gcs", "azblob"] as const;

function readOptionalBooleanAtPath(
  value: unknown,
  path: string,
  errors: string[]
): boolean | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (typeof value !== "boolean") {
    errors.push(`${path} must be a boolean`);
    return undefined;
  }
  return value;
}

function readOptionalNumberAtPath(
  value: unknown,
  path: string,
  errors: string[],
  minimum = 0
): number | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!isFiniteNumber(value)) {
    errors.push(`${path} must be a number`);
    return undefined;
  }
  if (value < minimum) {
    errors.push(`${path} must be >= ${minimum}`);
    return undefined;
  }
  return value;
}

function readStateConfigAtPath(
  value: unknown,
  path: string,
  errors: string[]
): ProjectConfig["state"] | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path} must be an object`);
    return undefined;
  }

  for (const key of Object.keys(value)) {
    if (!["backend", "keyPrefix", "lock", "local", "postgres", "s3", "gcs", "azblob"].includes(key)) {
      errors.push(`${path}.${key} is not a supported field`);
    }
  }

  const state: NonNullable<ProjectConfig["state"]> = {};
  if ("backend" in value) {
    if (typeof value.backend !== "string" || !supportedStateBackends.includes(value.backend as (typeof supportedStateBackends)[number])) {
      errors.push(`${path}.backend must be one of: ${supportedStateBackends.join(", ")}`);
    } else {
      state.backend = value.backend as NonNullable<ProjectConfig["state"]>["backend"];
    }
  }
  if ("keyPrefix" in value) {
    if (typeof value.keyPrefix !== "string" || value.keyPrefix.trim().length === 0) {
      errors.push(`${path}.keyPrefix must be a non-empty string`);
    } else {
      state.keyPrefix = value.keyPrefix.trim();
    }
  }

  if ("lock" in value) {
    if (!isRecord(value.lock)) {
      errors.push(`${path}.lock must be an object`);
    } else {
      state.lock = {};
      state.lock.enabled = readOptionalBooleanAtPath(value.lock.enabled, `${path}.lock.enabled`, errors);
      state.lock.timeoutSeconds = readOptionalNumberAtPath(
        value.lock.timeoutSeconds,
        `${path}.lock.timeoutSeconds`,
        errors,
        1
      );
      state.lock.heartbeatSeconds = readOptionalNumberAtPath(
        value.lock.heartbeatSeconds,
        `${path}.lock.heartbeatSeconds`,
        errors,
        1
      );
      state.lock.staleAfterSeconds = readOptionalNumberAtPath(
        value.lock.staleAfterSeconds,
        `${path}.lock.staleAfterSeconds`,
        errors,
        1
      );
    }
  }

  if ("local" in value) {
    if (!isRecord(value.local)) {
      errors.push(`${path}.local must be an object`);
    } else {
      state.local = {};
      if ("dir" in value.local) {
        if (typeof value.local.dir !== "string" || value.local.dir.trim().length === 0) {
          errors.push(`${path}.local.dir must be a non-empty string`);
        } else {
          state.local.dir = value.local.dir.trim();
        }
      }
    }
  }

  if ("postgres" in value) {
    if (!isRecord(value.postgres)) {
      errors.push(`${path}.postgres must be an object`);
    } else {
      state.postgres = {};
      if ("connectionStringEnv" in value.postgres) {
        if (
          typeof value.postgres.connectionStringEnv !== "string" ||
          value.postgres.connectionStringEnv.trim().length === 0
        ) {
          errors.push(`${path}.postgres.connectionStringEnv must be a non-empty string`);
        } else {
          state.postgres.connectionStringEnv = value.postgres.connectionStringEnv.trim();
        }
      }
      if ("schema" in value.postgres) {
        if (typeof value.postgres.schema !== "string" || value.postgres.schema.trim().length === 0) {
          errors.push(`${path}.postgres.schema must be a non-empty string`);
        } else {
          state.postgres.schema = value.postgres.schema.trim();
        }
      }
      if ("table" in value.postgres) {
        if (typeof value.postgres.table !== "string" || value.postgres.table.trim().length === 0) {
          errors.push(`${path}.postgres.table must be a non-empty string`);
        } else {
          state.postgres.table = value.postgres.table.trim();
        }
      }
    }
  }

  if ("s3" in value) {
    if (!isRecord(value.s3)) {
      errors.push(`${path}.s3 must be an object`);
    } else {
      state.s3 = {};
      for (const field of ["bucket", "region", "keyPrefix"] as const) {
        if (!(field in value.s3)) {
          continue;
        }
        const fieldValue = value.s3[field];
        if (typeof fieldValue !== "string" || fieldValue.trim().length === 0) {
          errors.push(`${path}.s3.${field} must be a non-empty string`);
          continue;
        }
        state.s3[field] = fieldValue.trim();
      }
      state.s3.useLockfile = readOptionalBooleanAtPath(
        value.s3.useLockfile,
        `${path}.s3.useLockfile`,
        errors
      );
    }
  }

  if ("gcs" in value) {
    if (!isRecord(value.gcs)) {
      errors.push(`${path}.gcs must be an object`);
    } else {
      state.gcs = {};
      for (const field of ["bucket", "prefix"] as const) {
        if (!(field in value.gcs)) {
          continue;
        }
        const fieldValue = value.gcs[field];
        if (typeof fieldValue !== "string" || fieldValue.trim().length === 0) {
          errors.push(`${path}.gcs.${field} must be a non-empty string`);
          continue;
        }
        state.gcs[field] = fieldValue.trim();
      }
    }
  }

  if ("azblob" in value) {
    if (!isRecord(value.azblob)) {
      errors.push(`${path}.azblob must be an object`);
    } else {
      state.azblob = {};
      for (const field of ["container", "prefix"] as const) {
        if (!(field in value.azblob)) {
          continue;
        }
        const fieldValue = value.azblob[field];
        if (typeof fieldValue !== "string" || fieldValue.trim().length === 0) {
          errors.push(`${path}.azblob.${field} must be a non-empty string`);
          continue;
        }
        state.azblob[field] = fieldValue.trim();
      }
    }
  }

  if (state.backend === "s3" && !state.s3?.bucket) {
    errors.push(`${path}.s3.bucket is required when state.backend is s3`);
  }
  if (state.backend === "gcs" && !state.gcs?.bucket) {
    errors.push(`${path}.gcs.bucket is required when state.backend is gcs`);
  }
  if (state.backend === "azblob" && !state.azblob?.container) {
    errors.push(`${path}.azblob.container is required when state.backend is azblob`);
  }

  return state;
}

function cloneExtensionValueAtPath(value: unknown, path: string, errors: string[]): unknown {
  if (isScalar(value)) {
    return value;
  }

  if (Array.isArray(value)) {
    return value.map((entry, index) =>
      cloneExtensionValueAtPath(entry, `${path}[${index}]`, errors)
    );
  }

  if (isRecord(value)) {
    const cloned: Record<string, unknown> = {};
    for (const [entryKey, entryValue] of Object.entries(value)) {
      cloned[entryKey] = cloneExtensionValueAtPath(entryValue, `${path}.${entryKey}`, errors);
    }
    return cloned;
  }

  errors.push(`${path} has an unsupported value`);
  return undefined;
}

function readExtensionsAtPath(
  value: unknown,
  path: string,
  errors: string[]
): ProjectConfig["extensions"] {
  if (value === undefined) {
    return undefined;
  }
  if (!isRecord(value)) {
    errors.push(`${path} must be an object`);
    return undefined;
  }

  const out: NonNullable<ProjectConfig["extensions"]> = {};
  for (const [provider, providerConfig] of Object.entries(value)) {
    if (!isRecord(providerConfig)) {
      errors.push(`${path}.${provider} must be an object`);
      continue;
    }

    const extensionValues: Record<string, unknown> = {};
    for (const [field, fieldValue] of Object.entries(providerConfig)) {
      const clonedValue = cloneExtensionValueAtPath(fieldValue, `${path}.${provider}.${field}`, errors);
      if (clonedValue !== undefined) {
        extensionValues[field] = clonedValue;
      }
    }
    out[provider] = extensionValues;
  }

  return out;
}

function readExtensions(
  source: Record<string, unknown>,
  errors: string[]
): ProjectConfig["extensions"] {
  return readExtensionsAtPath(source.extensions, "extensions", errors);
}

function mergeExtensions(
  base: ProjectConfig["extensions"],
  override: ProjectConfig["extensions"]
): ProjectConfig["extensions"] {
  if (!base && !override) {
    return undefined;
  }

  const merged: NonNullable<ProjectConfig["extensions"]> = {
    ...(base || {})
  };

  if (override) {
    for (const [provider, values] of Object.entries(override)) {
      if (!values || !isRecord(values)) {
        continue;
      }
      const current = merged[provider];
      merged[provider] = deepMergeRecords(isRecord(current) ? current : undefined, values);
    }
  }

  return merged;
}

function mergeStateConfig(
  base: ProjectConfig["state"],
  override: ProjectConfig["state"]
): ProjectConfig["state"] {
  if (!base && !override) {
    return undefined;
  }

  const merged = deepMergeRecords(
    (base || {}) as Record<string, unknown>,
    (override || {}) as Record<string, unknown>
  );
  return merged as NonNullable<ProjectConfig["state"]>;
}

function deepMergeRecords(
  base: Record<string, unknown> | undefined,
  override: Record<string, unknown> | undefined
): Record<string, unknown> {
  const merged: Record<string, unknown> = {
    ...(base || {})
  };

  if (!override) {
    return merged;
  }

  for (const [key, value] of Object.entries(override)) {
    const baseValue = merged[key];
    if (isRecord(baseValue) && isRecord(value)) {
      merged[key] = deepMergeRecords(baseValue, value);
    } else {
      merged[key] = value;
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
    secrets: {
      ...(base.secrets || {}),
      ...(override.secrets || {})
    },
    workflows: override.workflows || base.workflows,
    params: {
      ...(base.params || {}),
      ...(override.params || {})
    },
    extensions: mergeExtensions(base.extensions, override.extensions),
    state: mergeStateConfig(base.state, override.state)
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
  override.secrets = readSecretsAtPath(source.secrets, `${path}.secrets`, errors);
  override.workflows = readWorkflowsAtPath(source.workflows, `${path}.workflows`, errors);
  override.params = readStringRecordAtPath(source.params, `${path}.params`, errors);
  override.extensions = readExtensionsAtPath(source.extensions, `${path}.extensions`, errors);
  override.state = readStateConfigAtPath(source.state, `${path}.state`, errors);

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

function validateAwsIamStatementAtPath(
  statementValue: unknown,
  path: string,
  errors: string[]
): void {
  if (!isRecord(statementValue)) {
    errors.push(`${path} must be an object`);
    return;
  }

  const statement = statementValue;
  for (const key of Object.keys(statement)) {
    if (!["sid", "effect", "actions", "resources", "condition"].includes(key)) {
      errors.push(`${path}.${key} is not a supported field`);
    }
  }

  if ("sid" in statement && (typeof statement.sid !== "string" || statement.sid.trim().length === 0)) {
    errors.push(`${path}.sid must be a non-empty string`);
  }

  if (
    typeof statement.effect !== "string" ||
    ![AwsIamEffectEnum.Allow, AwsIamEffectEnum.Deny].includes(statement.effect as AwsIamEffectEnum)
  ) {
    errors.push(`${path}.effect must be Allow or Deny`);
  }

  if (!isStringArray(statement.actions) || statement.actions.length === 0) {
    errors.push(`${path}.actions must be an array with at least one action`);
  }

  if (!isStringArray(statement.resources) || statement.resources.length === 0) {
    errors.push(`${path}.resources must be an array with at least one resource`);
  }

  if ("condition" in statement && !isRecord(statement.condition)) {
    errors.push(`${path}.condition must be an object`);
  }
}

function validateAwsIamConfigAtPath(value: unknown, path: string, errors: string[]): void {
  if (!isRecord(value)) {
    errors.push(`${path} must be an object`);
    return;
  }

  for (const key of Object.keys(value)) {
    if (key !== "role") {
      errors.push(`${path}.${key} is not a supported field`);
    }
  }

  if (!("role" in value) || value.role === undefined) {
    return;
  }

  if (!isRecord(value.role)) {
    errors.push(`${path}.role must be an object`);
    return;
  }

  for (const key of Object.keys(value.role)) {
    if (key !== "statements") {
      errors.push(`${path}.role.${key} is not a supported field`);
    }
  }

  if (!("statements" in value.role) || value.role.statements === undefined) {
    return;
  }

  if (!Array.isArray(value.role.statements)) {
    errors.push(`${path}.role.statements must be an array`);
    return;
  }

  value.role.statements.forEach((statement, index) => {
    validateAwsIamStatementAtPath(statement, `${path}.role.statements[${index}]`, errors);
  });
}

function validateExtensionTypes(
  extensions: ProjectConfig["extensions"],
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

    if (!values || !isRecord(values)) {
      errors.push(`extensions.${provider} must be an object`);
      continue;
    }

    for (const [field, value] of Object.entries(values)) {
      const expectedType = schema[field];
      if (!expectedType) {
        errors.push(`extensions.${provider}.${field} is not a supported field`);
        continue;
      }

      if (expectedType === "object") {
        if (!isRecord(value)) {
          errors.push(`extensions.${provider}.${field} must be an object`);
          continue;
        }
        if (provider === "aws-lambda" && field === "iam") {
          validateAwsIamConfigAtPath(value, `extensions.${provider}.${field}`, errors);
        }
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
  const sourceCandidate = resolveDynamicBindingsAtPath(raw, "root", errors);
  if (!isRecord(sourceCandidate)) {
    throw new Error("Invalid runfabric.yml: root document must be an object");
  }
  const source = sourceCandidate;

  const service = readRequiredString(source, "service", errors);
  const runtime = readRequiredString(source, "runtime", errors);
  const entry = readRequiredString(source, "entry", errors);
  const providers = readStringArray(source, "providers", errors, 1);
  const triggers = readTriggerArray(source, errors);
  const functions = readFunctions(source, errors);
  const hooks = "hooks" in source ? readStringArrayAtPath(source.hooks, "hooks", errors) : undefined;
  const resources = readResourcesAtPath(source.resources, "resources", errors);
  const env = readStringRecord(source, "env", errors);
  const secrets = readSecretsAtPath(source.secrets, "secrets", errors);
  const workflows = readWorkflowsAtPath(source.workflows, "workflows", errors);
  const params = readStringRecord(source, "params", errors);
  const extensions = readExtensions(source, errors);
  const state = readStateConfigAtPath(source.state, "state", errors);
  const stageOverrides = readStageOverrides(source, errors);

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
    secrets,
    workflows,
    params,
    extensions,
    state,
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
