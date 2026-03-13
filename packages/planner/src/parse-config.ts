import type { ProjectConfig } from "@runfabric/core";
import { readExtensions, validateExtensionTypes } from "./parse-config/extensions";
import {
  readFunctions,
  readResourcesAtPath,
  readSecretsAtPath,
  readWorkflowsAtPath
} from "./parse-config/project-readers";
import {
  isRecord,
  readRequiredString,
  readStringArray,
  readStringArrayAtPath,
  readStringRecord,
  resolveDynamicBindingsAtPath
} from "./parse-config/shared";
import { readStageOverrides, applyStageOverride, type StageOverride } from "./parse-config/stages";
import { readStateConfigAtPath } from "./parse-config/state-config";
import { readTriggerArray } from "./parse-config/triggers";
import { parseYamlDocument } from "./parse-config/yaml";
import { readDeployConfigAtPath } from "./parse-config/deploy-config";
import { readOptionalRuntimeModeAtPath, readRequiredRuntimeAtPath } from "./parse-config/runtime";
import {
  applyRuntimeEntryOverride,
  createRuntimeEntryContext,
  validateFunctionRuntimeEntries,
  validateRuntimeEntryContext
} from "./parse-config/runtime-entry";

export interface ParseProjectConfigOptions {
  stage?: string;
}

function resolveSelectedStage(options: ParseProjectConfigOptions): string {
  return (options.stage || process.env.RUNFABRIC_STAGE || "default").trim() || "default";
}

function readProjectSections(source: Record<string, unknown>, errors: string[]) {
  return {
    providers: readStringArray(source, "providers", errors, 1),
    triggers: readTriggerArray(source, errors),
    functions: readFunctions(source, errors),
    hooks: "hooks" in source ? readStringArrayAtPath(source.hooks, "hooks", errors) : undefined,
    resources: readResourcesAtPath(source.resources, "resources", errors),
    env: readStringRecord(source, "env", errors),
    secrets: readSecretsAtPath(source.secrets, "secrets", errors),
    workflows: readWorkflowsAtPath(source.workflows, "workflows", errors),
    params: readStringRecord(source, "params", errors),
    extensions: readExtensions(source, errors),
    deploy: readDeployConfigAtPath(source.deploy, "deploy", errors),
    state: readStateConfigAtPath(source.state, "state", errors)
  };
}

function applySelectedStageOverride(
  project: ProjectConfig,
  stageOverrides: Record<string, StageOverride> | undefined,
  selectedStage: string,
  errors: string[]
): ProjectConfig {
  if (!stageOverrides) {
    return project;
  }

  let nextProject = project;
  if (stageOverrides.default) {
    nextProject = applyStageOverride(nextProject, stageOverrides.default);
  }

  if (selectedStage !== "default") {
    const selectedOverride = stageOverrides[selectedStage];
    if (!selectedOverride) {
      errors.push(`stages.${selectedStage} is not defined`);
    } else {
      nextProject = applyStageOverride(nextProject, selectedOverride);
    }
  }

  return nextProject;
}

function validateRootRuntimeEntries(project: ProjectConfig, errors: string[]): void {
  const rootContext = createRuntimeEntryContext(project.runtime, project.entry, "runtime", "entry");
  validateRuntimeEntryContext(rootContext, errors);
  validateFunctionRuntimeEntries(project.functions, "functions", rootContext, errors);
}

function validateStageRuntimeEntries(
  project: ProjectConfig,
  stageOverrides: Record<string, StageOverride> | undefined,
  errors: string[]
): void {
  if (!stageOverrides) {
    return;
  }

  const rootContext = createRuntimeEntryContext(project.runtime, project.entry, "runtime", "entry");
  const defaultOverride = stageOverrides.default;
  const defaultContext = defaultOverride
    ? applyRuntimeEntryOverride({
        base: rootContext,
        runtime: defaultOverride.runtime,
        entry: defaultOverride.entry,
        runtimePath: "stages.default.runtime",
        entryPath: "stages.default.entry"
      })
    : rootContext;

  if (defaultOverride) {
    validateRuntimeEntryContext(defaultContext, errors);
    validateFunctionRuntimeEntries(defaultOverride.functions, "stages.default.functions", defaultContext, errors);
  }

  for (const [stageName, override] of Object.entries(stageOverrides)) {
    if (stageName === "default") {
      continue;
    }
    const stageContext = applyRuntimeEntryOverride({
      base: defaultContext,
      runtime: override.runtime,
      entry: override.entry,
      runtimePath: `stages.${stageName}.runtime`,
      entryPath: `stages.${stageName}.entry`
    });
    if (override.runtime || override.entry) {
      validateRuntimeEntryContext(stageContext, errors);
    }
    validateFunctionRuntimeEntries(override.functions, `stages.${stageName}.functions`, stageContext, errors);
  }
}

function validateProjectConfigShape(raw: unknown, options: ParseProjectConfigOptions = {}): ProjectConfig {
  if (!isRecord(raw)) {
    throw new Error("Invalid runfabric.yml: root document must be an object");
  }

  const errors: string[] = [];
  const sourceCandidate = resolveDynamicBindingsAtPath(raw, "root", errors);
  if (!isRecord(sourceCandidate)) {
    throw new Error("Invalid runfabric.yml: root document must be an object");
  }

  const service = readRequiredString(sourceCandidate, "service", errors);
  const runtime = readRequiredRuntimeAtPath(readRequiredString(sourceCandidate, "runtime", errors), "runtime", errors);
  const runtimeMode = readOptionalRuntimeModeAtPath(sourceCandidate.runtimeMode, "runtimeMode", errors);
  const entry = readRequiredString(sourceCandidate, "entry", errors);
  const selectedStage = resolveSelectedStage(options);
  const stageOverrides = readStageOverrides(sourceCandidate, errors);
  const sections = readProjectSections(sourceCandidate, errors);

  let project: ProjectConfig = {
    service,
    runtime: runtime || "nodejs",
    runtimeMode,
    entry,
    ...sections,
    stage: selectedStage
  };

  validateRootRuntimeEntries(project, errors);
  validateStageRuntimeEntries(project, stageOverrides, errors);
  project = applySelectedStageOverride(project, stageOverrides, selectedStage, errors);
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
  return validateProjectConfigShape(parseYaml(content), options);
}

export function parseYaml(content: string): unknown {
  return parseYamlDocument(content);
}
