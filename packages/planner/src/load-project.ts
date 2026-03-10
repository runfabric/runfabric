import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import type { ProjectConfig } from "@runfabric/core";
import { parseProjectConfig, type ParseProjectConfigOptions } from "./parse-config";
import { createPlan, type PlanningResult } from "./planner";

export async function loadProjectConfig(
  configPath: string,
  options: ParseProjectConfigOptions = {}
): Promise<ProjectConfig> {
  const fileContent = await readFile(resolve(configPath), "utf8");
  return parseProjectConfig(fileContent, options);
}

export async function loadAndPlanProject(
  configPath: string,
  options: ParseProjectConfigOptions = {}
): Promise<PlanningResult> {
  const project = await loadProjectConfig(configPath, options);
  return createPlan(project);
}
