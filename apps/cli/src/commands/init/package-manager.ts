import type { PackageManager } from "./types";

export function detectPackageManager(userAgent: string | undefined): PackageManager {
  const normalizedAgent = (userAgent || "").toLowerCase();
  if (normalizedAgent.includes("pnpm")) {
    return "pnpm";
  }
  if (normalizedAgent.includes("yarn")) {
    return "yarn";
  }
  if (normalizedAgent.includes("bun")) {
    return "bun";
  }
  return "npm";
}

export function parsePackageManager(value: string): PackageManager | undefined {
  if (value === "npm" || value === "pnpm" || value === "yarn" || value === "bun") {
    return value;
  }
  return undefined;
}

export function packageManagerRunArgs(
  packageManager: PackageManager,
  scriptName: string
): [string, string[]] {
  if (packageManager === "yarn") {
    return ["yarn", [scriptName]];
  }
  return [packageManager, ["run", scriptName]];
}
