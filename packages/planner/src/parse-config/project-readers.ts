import type { FunctionConfig, ProjectConfig } from "@runfabric/core";
import {
  isRecord,
  isScalar,
  readOptionalNumberAtPath,
  readOptionalString,
  readRequiredString,
  readStringRecordAtPath
} from "./shared";
import { readOptionalRuntimeAtPath } from "./runtime";
import { readTriggerArrayAtPath } from "./triggers";

function cloneSupportedValueAtPath(value: unknown, path: string, errors: string[]): unknown {
  if (isScalar(value)) {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map((entry, index) => cloneSupportedValueAtPath(entry, `${path}[${index}]`, errors));
  }
  if (isRecord(value)) {
    const cloned: Record<string, unknown> = {};
    for (const [entryKey, entryValue] of Object.entries(value)) {
      cloned[entryKey] = cloneSupportedValueAtPath(entryValue, `${path}.${entryKey}`, errors);
    }
    return cloned;
  }

  errors.push(`${path} has an unsupported value`);
  return undefined;
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
  for (let index = 0; index < value.length; index += 1) {
    const item = value[index];
    if (!isRecord(item)) {
      errors.push(`${path}[${index}] must be an object`);
      continue;
    }
    if (typeof item.name !== "string" || item.name.trim().length === 0) {
      errors.push(`${path}[${index}].name must be a non-empty string`);
      continue;
    }

    const normalized: { name: string; [key: string]: unknown } = { name: item.name.trim() };
    for (const [key, rawValue] of Object.entries(item)) {
      if (key === "name") {
        continue;
      }
      const cloned = cloneSupportedValueAtPath(rawValue, `${path}[${index}].${key}`, errors);
      if (cloned !== undefined) {
        normalized[key] = cloned;
      }
    }
    resources.push(normalized);
  }

  return resources;
}

export function readResourcesAtPath(
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

  const hasAnyField =
    resources.memory !== undefined ||
    resources.timeout !== undefined ||
    resources.queues !== undefined ||
    resources.buckets !== undefined ||
    resources.topics !== undefined ||
    resources.databases !== undefined;

  return hasAnyField ? resources : undefined;
}

export function readFunctionsAtPath(
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
  for (let index = 0; index < value.length; index += 1) {
    const item = value[index];
    if (!isRecord(item)) {
      errors.push(`${path}[${index}] must be an object`);
      continue;
    }

    const name = readRequiredString(item, "name", errors);
    if (!name) {
      continue;
    }

    functions.push({
      name,
      entry: readOptionalString(item, "entry", errors),
      runtime: readOptionalRuntimeAtPath(item.runtime, `${path}[${index}].runtime`, errors),
      triggers: "triggers" in item
        ? readTriggerArrayAtPath(item.triggers, `${path}[${index}].triggers`, errors, 1)
        : undefined,
      resources: readResourcesAtPath(item.resources, `${path}[${index}].resources`, errors),
      env: readStringRecordAtPath(item.env, `${path}[${index}].env`, errors)
    });
  }

  return functions.length > 0 ? functions : undefined;
}

export function readFunctions(
  source: Record<string, unknown>,
  errors: string[]
): FunctionConfig[] | undefined {
  return readFunctionsAtPath(source.functions, "functions", errors);
}

function parseWorkflowRetry(
  value: unknown,
  stepPath: string,
  errors: string[]
): { attempts?: number; backoffSeconds?: number } | undefined {
  if (!isRecord(value)) {
    errors.push(`${stepPath}.retry must be an object`);
    return undefined;
  }

  const retry: { attempts?: number; backoffSeconds?: number } = {};
  const attempts = readOptionalNumberAtPath(value.attempts, `${stepPath}.retry.attempts`, errors, 1);
  const backoffSeconds = readOptionalNumberAtPath(
    value.backoffSeconds,
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
  return Object.keys(retry).length > 0 ? retry : undefined;
}

function parseWorkflowStep(
  step: unknown,
  workflowPath: string,
  stepIndex: number,
  errors: string[]
): NonNullable<ProjectConfig["workflows"]>[number]["steps"][number] | undefined {
  if (!isRecord(step)) {
    errors.push(`${workflowPath}.steps[${stepIndex}] must be an object`);
    return undefined;
  }

  const stepPath = `${workflowPath}.steps[${stepIndex}]`;
  const fn = readRequiredString(step, "function", errors);
  if (!fn) {
    return undefined;
  }

  const normalized: NonNullable<ProjectConfig["workflows"]>[number]["steps"][number] = {
    function: fn
  };
  const next = readOptionalString(step, "next", errors);
  if (next) {
    normalized.next = next;
  }

  if ("retry" in step) {
    const retry = parseWorkflowRetry(step.retry, stepPath, errors);
    if (retry) {
      normalized.retry = retry;
    }
  }

  const timeoutSeconds = readOptionalNumberAtPath(step.timeoutSeconds, `${stepPath}.timeoutSeconds`, errors, 1);
  if (timeoutSeconds !== undefined) {
    normalized.timeoutSeconds = timeoutSeconds;
  }

  return normalized;
}

function parseWorkflow(
  item: unknown,
  path: string,
  index: number,
  errors: string[]
): NonNullable<ProjectConfig["workflows"]>[number] | undefined {
  if (!isRecord(item)) {
    errors.push(`${path}[${index}] must be an object`);
    return undefined;
  }

  const workflowPath = `${path}[${index}]`;
  const name = readRequiredString(item, "name", errors);
  if (!Array.isArray(item.steps)) {
    errors.push(`${workflowPath}.steps must be an array`);
    return undefined;
  }

  const steps: NonNullable<ProjectConfig["workflows"]>[number]["steps"] = [];
  for (let stepIndex = 0; stepIndex < item.steps.length; stepIndex += 1) {
    const parsed = parseWorkflowStep(item.steps[stepIndex], workflowPath, stepIndex, errors);
    if (parsed) {
      steps.push(parsed);
    }
  }

  if (!name) {
    return undefined;
  }
  if (steps.length === 0) {
    errors.push(`${workflowPath}.steps must contain at least one step`);
    return undefined;
  }

  return { name, steps };
}

export function readWorkflowsAtPath(
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
  for (let index = 0; index < value.length; index += 1) {
    const workflow = parseWorkflow(value[index], path, index, errors);
    if (workflow) {
      workflows.push(workflow);
    }
  }

  return workflows.length > 0 ? workflows : undefined;
}

export function readSecretsAtPath(
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
