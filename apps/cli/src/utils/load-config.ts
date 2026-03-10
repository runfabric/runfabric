import type { ProjectConfig } from "@runfabric/core";
import { loadAndPlanProject, type PlanningResult } from "@runfabric/planner";
import { resolveConfigPath } from "./resolve-project";

export interface PlanningContext {
  configPath: string;
  project: ProjectConfig;
  planning: PlanningResult;
}

export async function loadPlanningContext(
  projectDir: string,
  configOption?: string,
  stage?: string
): Promise<PlanningContext> {
  const configPath = resolveConfigPath(projectDir, configOption);
  const planning = await loadAndPlanProject(configPath, { stage });
  return {
    configPath,
    project: planning.project,
    planning
  };
}
