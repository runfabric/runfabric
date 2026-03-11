import type { ProjectConfig } from "@runfabric/core";

export function listProjectFunctionNames(project: ProjectConfig): string[] {
  if (project.functions && project.functions.length > 0) {
    return project.functions.map((fn) => fn.name);
  }
  return ["default", project.service];
}

export function resolveFunctionProject(project: ProjectConfig, functionName?: string): ProjectConfig {
  if (!functionName) {
    return project;
  }

  if (!project.functions || project.functions.length === 0) {
    if (functionName === "default" || functionName === project.service) {
      return project;
    }
    throw new Error(
      `unknown function "${functionName}". Available: default, ${project.service}`
    );
  }

  const target = project.functions.find((fn) => fn.name === functionName);
  if (!target) {
    throw new Error(
      `unknown function "${functionName}". Available: ${project.functions
        .map((fn) => fn.name)
        .join(", ")}`
    );
  }

  return {
    ...project,
    runtime: target.runtime || project.runtime,
    entry: target.entry || project.entry,
    triggers: target.triggers || project.triggers,
    resources: target.resources || project.resources,
    env: {
      ...(project.env || {}),
      ...(target.env || {})
    }
  };
}
