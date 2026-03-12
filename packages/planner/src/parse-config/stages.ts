import type { FunctionConfig, ProjectConfig, TriggerConfig } from "@runfabric/core";
import { mergeExtensions, mergeStateConfig, readExtensionsAtPath } from "./extensions";
import {
  isRecord,
  readOptionalString,
  readStringArrayAtPath,
  readStringRecordAtPath
} from "./shared";
import { readStateConfigAtPath } from "./state-config";
import {
  readFunctionsAtPath,
  readResourcesAtPath,
  readSecretsAtPath,
  readWorkflowsAtPath
} from "./project-readers";
import { readTriggerArrayAtPath } from "./triggers";

export interface StageOverride {
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

export function applyStageOverride(base: ProjectConfig, override: StageOverride): ProjectConfig {
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
    env: { ...(base.env || {}), ...(override.env || {}) },
    secrets: { ...(base.secrets || {}), ...(override.secrets || {}) },
    workflows: override.workflows || base.workflows,
    params: { ...(base.params || {}), ...(override.params || {}) },
    extensions: mergeExtensions(base.extensions, override.extensions),
    state: mergeStateConfig(base.state, override.state)
  };
}

export function readStageOverride(
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

export function readStageOverrides(
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
