import { basename, extname } from "node:path";
import type { FunctionConfig, RuntimeFamily } from "@runfabric/core";

export interface RuntimeEntryContext {
  runtime: RuntimeFamily;
  entry: string;
  runtimePath: string;
  entryPath: string;
}

const runtimeEntryHints: Record<RuntimeFamily, string> = {
  nodejs: ".js | .mjs | .cjs | .ts | .tsx | .mts | .cts | .jsx",
  python: ".py",
  go: ".go or extensionless compiled binary",
  java: ".java | .jar",
  rust: ".rs or extensionless compiled binary",
  dotnet: ".cs | .csproj | .sln | .dll or extensionless binary"
};

function isCompatibleEntry(runtime: RuntimeFamily, entry: string): boolean {
  const extension = extname(basename(entry.trim().toLowerCase()));
  if (!extension && (runtime === "go" || runtime === "rust" || runtime === "dotnet")) {
    return true;
  }
  if (runtime === "nodejs") {
    return [".js", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts", ".jsx"].includes(extension);
  }
  if (runtime === "python") {
    return extension === ".py";
  }
  if (runtime === "go") {
    return extension === ".go";
  }
  if (runtime === "java") {
    return extension === ".java" || extension === ".jar";
  }
  if (runtime === "rust") {
    return extension === ".rs";
  }
  return [".cs", ".csproj", ".sln", ".dll"].includes(extension);
}

export function createRuntimeEntryContext(
  runtime: RuntimeFamily,
  entry: string,
  runtimePath: string,
  entryPath: string
): RuntimeEntryContext {
  return { runtime, entry, runtimePath, entryPath };
}

export function applyRuntimeEntryOverride(input: {
  base: RuntimeEntryContext;
  runtime?: RuntimeFamily;
  entry?: string;
  runtimePath: string;
  entryPath: string;
}): RuntimeEntryContext {
  return {
    runtime: input.runtime || input.base.runtime,
    entry: input.entry || input.base.entry,
    runtimePath: input.runtime ? input.runtimePath : input.base.runtimePath,
    entryPath: input.entry ? input.entryPath : input.base.entryPath
  };
}

export function validateRuntimeEntryContext(context: RuntimeEntryContext, errors: string[]): void {
  if (isCompatibleEntry(context.runtime, context.entry)) {
    return;
  }
  errors.push(
    `${context.entryPath} (${context.entry}) is not compatible with ${context.runtimePath} (${context.runtime}); expected ${runtimeEntryHints[context.runtime]}`
  );
}

export function validateFunctionRuntimeEntries(
  functions: FunctionConfig[] | undefined,
  pathPrefix: string,
  context: RuntimeEntryContext,
  errors: string[]
): void {
  if (!functions || functions.length === 0) {
    return;
  }

  for (let index = 0; index < functions.length; index += 1) {
    const fn = functions[index];
    const functionPath = `${pathPrefix}[${index}]`;
    const runtime = fn.runtime || context.runtime;
    const entry = fn.entry || context.entry;
    const runtimePath = fn.runtime ? `${functionPath}.runtime` : context.runtimePath;
    const entryPath = fn.entry ? `${functionPath}.entry` : context.entryPath;
    if (isCompatibleEntry(runtime, entry)) {
      continue;
    }
    errors.push(
      `function ${fn.name}: ${entryPath} (${entry}) is not compatible with ${runtimePath} (${runtime}); expected ${runtimeEntryHints[runtime]}`
    );
  }
}
