import type { LifecycleHook } from "@runfabric/core";
import type { ProjectConfig } from "@runfabric/core";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { warn } from "./logger";

function toHookCandidate(moduleValue: unknown): LifecycleHook | null {
  if (!moduleValue || typeof moduleValue !== "object") {
    return null;
  }
  return moduleValue as LifecycleHook;
}

export async function loadLifecycleHooks(
  project: ProjectConfig,
  projectDir: string
): Promise<LifecycleHook[]> {
  if (!project.hooks || project.hooks.length === 0) {
    return [];
  }

  const loaded: LifecycleHook[] = [];
  for (const hookPath of project.hooks) {
    const absolutePath = resolve(projectDir, hookPath);
    const hookUrl = pathToFileURL(absolutePath).href;
    const moduleNamespace = await import(hookUrl);

    const defaultExport = toHookCandidate(moduleNamespace.default);
    if (defaultExport) {
      loaded.push(defaultExport);
      continue;
    }

    const moduleAsHook = toHookCandidate(moduleNamespace);
    if (moduleAsHook) {
      loaded.push(moduleAsHook);
      continue;
    }

    warn(`hook ${hookPath} did not export a recognized hook object`);
  }

  return loaded;
}

