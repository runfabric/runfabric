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
import { readStageOverrides, applyStageOverride } from "./parse-config/stages";
import { readStateConfigAtPath } from "./parse-config/state-config";
import { readTriggerArray } from "./parse-config/triggers";
import { parseYamlDocument } from "./parse-config/yaml";
import { readDeployConfigAtPath } from "./parse-config/deploy-config";
import { readRequiredRuntimeAtPath } from "./parse-config/runtime";

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
  source: Record<string, unknown>,
  selectedStage: string,
  errors: string[]
): ProjectConfig {
  const stageOverrides = readStageOverrides(source, errors);
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
  const entry = readRequiredString(sourceCandidate, "entry", errors);
  const selectedStage = resolveSelectedStage(options);
  const sections = readProjectSections(sourceCandidate, errors);

  let project: ProjectConfig = {
    service,
    runtime: runtime || "nodejs",
    entry,
    ...sections,
    stage: selectedStage
  };

  project = applySelectedStageOverride(project, sourceCandidate, selectedStage, errors);
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
