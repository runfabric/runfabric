export const RUNTIME_FAMILIES = [
  "nodejs",
  "python",
  "go",
  "java",
  "rust",
  "dotnet"
] as const;

export type RuntimeFamily = (typeof RUNTIME_FAMILIES)[number];

export const RUNTIME_MODES = ["native-compat", "engine"] as const;

const runtimeAliases: Record<string, RuntimeFamily> = {
  node: "nodejs",
  nodejs: "nodejs",
  javascript: "nodejs",
  typescript: "nodejs",
  python: "python",
  py: "python",
  go: "go",
  golang: "go",
  java: "java",
  rust: "rust",
  dotnet: "dotnet",
  "dot-net": "dotnet",
  "c#": "dotnet"
};

export function normalizeRuntimeFamily(value: string): RuntimeFamily | undefined {
  const normalized = value.trim().toLowerCase();
  if (normalized.startsWith("node")) {
    return "nodejs";
  }
  if (normalized.startsWith("python") || normalized.startsWith("py")) {
    return "python";
  }
  if (/^go(?:\d|$)/.test(normalized)) {
    return "go";
  }
  if (/^java(?:\d|$)/.test(normalized)) {
    return "java";
  }
  if (/^rust(?:\d|$)/.test(normalized)) {
    return "rust";
  }
  if (normalized.startsWith("dotnet") || normalized.startsWith("dot-net")) {
    return "dotnet";
  }
  return runtimeAliases[normalized];
}

export function isRuntimeFamily(value: string): value is RuntimeFamily {
  const normalized = value.trim().toLowerCase();
  return RUNTIME_FAMILIES.includes(normalized as RuntimeFamily);
}

export function runtimeFamilyList(): string {
  return RUNTIME_FAMILIES.join(" | ");
}

export function normalizeRuntimeMode(value: string): (typeof RUNTIME_MODES)[number] | undefined {
  const normalized = value.trim().toLowerCase();
  if (normalized === "native" || normalized === "native-compat") {
    return "native-compat";
  }
  if (normalized === "engine") {
    return "engine";
  }
  return undefined;
}

export function runtimeModeList(): string {
  return RUNTIME_MODES.join(" | ");
}
