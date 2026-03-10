import { access } from "node:fs/promises";
import { constants } from "node:fs";
import { resolve } from "node:path";

export function resolveConfigPath(projectDir: string, configOption?: string): string {
  if (configOption) {
    return resolve(projectDir, configOption);
  }
  return resolve(projectDir, "runfabric.yml");
}

export async function resolveProjectDir(startDir = process.cwd(), configOption?: string): Promise<string> {
  const startPath = resolve(startDir);

  if (configOption) {
    const configPath = resolveConfigPath(startPath, configOption);
    await access(configPath, constants.F_OK);
    return resolve(configPath, "..");
  }

  const configPath = resolveConfigPath(startPath);
  await access(configPath, constants.F_OK);
  return startPath;
}
