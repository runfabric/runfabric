import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { basename, extname, join, resolve } from "node:path";
import type { BuildArtifact, BuildResult, ProjectConfig } from "@runfabric/core";
import type { PlanningResult } from "@runfabric/planner";

export interface BuildProjectInput {
  planning: PlanningResult;
  project: ProjectConfig;
  projectDir: string;
  outputRoot?: string;
}

const MAX_PARALLEL_PROVIDER_ARTIFACTS = 4;

interface GeneratedFile {
  path: string;
  bytes: number;
  sha256: string;
  role: "entry-source" | "runtime-wrapper" | "manifest";
}

const requireModule = createRequire(__filename);

type TypeScriptModule = {
  ModuleKind: {
    ES2022: number;
  };
  ScriptTarget: {
    ES2022: number;
  };
  transpileModule: (
    input: string,
    options: {
      compilerOptions: {
        target: number;
        module: number;
        sourceMap: boolean;
        declaration: boolean;
        removeComments: boolean;
        esModuleInterop: boolean;
      };
      fileName: string;
      reportDiagnostics: boolean;
    }
  ) => { outputText: string };
};

interface SourceEntryInfo {
  sourceEntryPath: string;
  sourceEntryName: string;
  copiedEntryName: string;
  shouldTranspile: boolean;
}

interface MaterializedSourceEntry {
  copiedEntryName: string;
  content: string;
}

function toProviderSlug(provider: string): string {
  return provider.replace(/[^a-z0-9]/gi, "_");
}

function runtimeWrapperFilename(provider: string): string {
  if (provider === "cloudflare-workers") {
    return "worker.mjs";
  }
  if (provider === "aws-lambda") {
    return "lambda-handler.mjs";
  }
  if (provider === "gcp-functions") {
    return "gcp-handler.mjs";
  }
  if (provider === "azure-functions") {
    return "azure-handler.mjs";
  }
  return `${toProviderSlug(provider)}-handler.mjs`;
}

function isNodeLikeRuntime(runtime: string): boolean {
  const normalized = runtime.trim().toLowerCase();
  return normalized === "nodejs" || normalized === "node" || normalized.startsWith("node");
}

function createCloudflareWrapperContent(importSource: string, service: string): string {
  return [
    `import * as userModule from "${importSource}";`,
    "",
    "async function resolveResponse(request) {",
    "  const handler = userModule.handler || userModule.default;",
    "  if (typeof handler !== \"function\") {",
    `    return new Response("runfabric:${service}", { status: 200 });`,
    "  }",
    "  const result = await handler({",
    "    method: request.method,",
    "    path: new URL(request.url).pathname,",
    "    headers: Object.fromEntries(request.headers.entries())",
    "  });",
    "  if (result instanceof Response) {",
    "    return result;",
    "  }",
    "  if (result && typeof result === \"object\" && \"status\" in result) {",
    "    return new Response(result.body ?? \"\", {",
    "      status: Number(result.status) || 200,",
    "      headers: result.headers || {}",
    "    });",
    "  }",
    "  return new Response(JSON.stringify(result ?? {}), {",
    "    status: 200,",
    "    headers: { \"content-type\": \"application/json\" }",
    "  });",
    "}",
    "",
    "export default {",
    "  async fetch(request) {",
    "    return resolveResponse(request);",
    "  }",
    "};",
    ""
  ].join("\n");
}

function wrapperRuntimeName(provider: string): string {
  if (provider === "aws-lambda") {
    return "aws-lambda";
  }
  if (provider === "gcp-functions") {
    return "gcp-functions";
  }
  if (provider === "azure-functions") {
    return "azure-functions";
  }
  return "generic";
}

function createNodeWrapperContent(importSource: string, provider: string): string {
  const runtimeName = wrapperRuntimeName(provider);
  return [
    `import * as userModule from "${importSource}";`,
    "",
    "async function resolveHandler() {",
    "  const handler = userModule.handler || userModule.default;",
    "  if (typeof handler !== \"function\") {",
    "    return async () => ({",
    "      statusCode: 200,",
    `      body: JSON.stringify({ ok: true, provider: "${runtimeName}" })`,
    "    });",
    "  }",
    "  return handler;",
    "}",
    "",
    "export async function handler(event, context) {",
    "  const fn = await resolveHandler();",
    "  return fn(event, context);",
    "}",
    ""
  ].join("\n");
}

function createRuntimeWrapperContent(
  provider: string,
  sourceEntryRelativePath: string,
  service: string
): string {
  const importSource = `./${sourceEntryRelativePath}`;
  if (provider === "cloudflare-workers") {
    return createCloudflareWrapperContent(importSource, service);
  }
  return createNodeWrapperContent(importSource, provider);
}

function hashContent(content: string): string {
  return createHash("sha256").update(content).digest("hex");
}

async function readFileAsUtf8(filePath: string): Promise<string> {
  return readFile(filePath, "utf8");
}

function loadTypeScriptModule(): TypeScriptModule {
  try {
    return requireModule("typescript") as TypeScriptModule;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "MODULE_NOT_FOUND") {
      throw new Error(
        "typescript dependency is required to transpile TypeScript entries in build artifacts. Install it with: pnpm add -w --filter @runfabric/builder typescript"
      );
    }
    throw error;
  }
}

function transpileTypeScriptSource(source: string, fileName: string): string {
  const ts = loadTypeScriptModule();

  const result = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ES2022,
      sourceMap: false,
      declaration: false,
      removeComments: false,
      esModuleInterop: true
    },
    fileName,
    reportDiagnostics: false
  });

  return result.outputText;
}

function resolveSourceEntryInfo(project: ProjectConfig, projectDir: string): SourceEntryInfo {
  const sourceEntryPath = resolve(projectDir, project.entry);
  const sourceEntryName = basename(project.entry);
  const sourceEntryExt = extname(sourceEntryName).toLowerCase();
  const shouldTranspile = isNodeLikeRuntime(project.runtime) && (sourceEntryExt === ".ts" || sourceEntryExt === ".tsx");
  const copiedEntryName = shouldTranspile
    ? sourceEntryName.replace(/\.(ts|tsx)$/i, ".js")
    : sourceEntryName;

  return {
    sourceEntryPath,
    sourceEntryName,
    copiedEntryName,
    shouldTranspile
  };
}

async function materializeSourceEntry(entryInfo: SourceEntryInfo): Promise<MaterializedSourceEntry> {
  const sourceContent = await readFileAsUtf8(entryInfo.sourceEntryPath);
  if (entryInfo.shouldTranspile) {
    return {
      copiedEntryName: entryInfo.copiedEntryName,
      content: transpileTypeScriptSource(sourceContent, entryInfo.sourceEntryName)
    };
  }

  return {
    copiedEntryName: entryInfo.copiedEntryName,
    content: sourceContent
  };
}

function createGeneratedEntrySource(path: string, content: string): GeneratedFile {
  return {
    path,
    bytes: Buffer.byteLength(content, "utf8"),
    sha256: hashContent(content),
    role: "entry-source"
  };
}

async function addRuntimeWrapperFile(
  provider: string,
  project: ProjectConfig,
  runtimeDir: string,
  copiedEntryName: string,
  generatedFiles: GeneratedFile[]
): Promise<string | undefined> {
  if (!isNodeLikeRuntime(project.runtime)) {
    return undefined;
  }

  await mkdir(runtimeDir, { recursive: true });
  const runtimeFileName = runtimeWrapperFilename(provider);
  const runtimeFilePath = join(runtimeDir, runtimeFileName);
  const relativeSourceFromRuntime = `../src/${copiedEntryName}`;
  const wrapperContent = createRuntimeWrapperContent(provider, relativeSourceFromRuntime, project.service);
  await writeFile(runtimeFilePath, wrapperContent, "utf8");
  generatedFiles.push({
    path: runtimeFilePath,
    bytes: Buffer.byteLength(wrapperContent, "utf8"),
    sha256: hashContent(wrapperContent),
    role: "runtime-wrapper"
  });
  return runtimeFilePath;
}

async function writeArtifactManifest(
  provider: string,
  project: ProjectConfig,
  manifestPath: string,
  generatedFiles: GeneratedFile[]
): Promise<void> {
  const files = [...generatedFiles];
  const manifestContent = {
    provider,
    service: project.service,
    runtime: project.runtime,
    entry: project.entry,
    buildVersion: 1,
    generatedAt: new Date().toISOString(),
    files
  };
  const manifestJson = JSON.stringify(manifestContent, null, 2);
  await writeFile(manifestPath, manifestJson, "utf8");
  generatedFiles.push({
    path: manifestPath,
    bytes: Buffer.byteLength(manifestJson, "utf8"),
    sha256: hashContent(manifestJson),
    role: "manifest"
  });
}

async function buildProviderArtifact(
  provider: string,
  project: ProjectConfig,
  outputRoot: string,
  sourceEntry: MaterializedSourceEntry
): Promise<BuildArtifact> {
  const providerRoot = join(outputRoot, provider, project.service);
  const sourceDir = join(providerRoot, "src");
  const runtimeDir = join(providerRoot, "runtime");
  const manifestPath = join(providerRoot, "artifact.json");
  const copiedEntryPath = join(sourceDir, sourceEntry.copiedEntryName);

  await mkdir(sourceDir, { recursive: true });
  await writeFile(copiedEntryPath, sourceEntry.content, "utf8");
  const generatedFiles: GeneratedFile[] = [createGeneratedEntrySource(copiedEntryPath, sourceEntry.content)];
  const runtimeEntry = await addRuntimeWrapperFile(
    provider,
    project,
    runtimeDir,
    sourceEntry.copiedEntryName,
    generatedFiles
  );
  await writeArtifactManifest(provider, project, manifestPath, generatedFiles);

  return {
    provider,
    entry: runtimeEntry || copiedEntryPath,
    outputPath: manifestPath
  };
}

export async function buildProject(input: BuildProjectInput): Promise<BuildResult> {
  const outputRoot = input.outputRoot || resolve(input.projectDir, ".runfabric", "build");
  const activePlans = input.planning.providerPlans.filter((providerPlan) => providerPlan.errors.length === 0);

  if (activePlans.length === 0) {
    return { artifacts: [] };
  }

  const entryInfo = resolveSourceEntryInfo(input.project, input.projectDir);
  const materializedEntry = await materializeSourceEntry(entryInfo);
  const artifacts: Array<BuildArtifact | undefined> = new Array(activePlans.length);
  const workerCount = Math.min(MAX_PARALLEL_PROVIDER_ARTIFACTS, activePlans.length);
  let nextIndex = 0;

  const workers = Array.from({ length: workerCount }, async () => {
    while (true) {
      const currentIndex = nextIndex;
      nextIndex += 1;
      if (currentIndex >= activePlans.length) {
        return;
      }

      const providerPlan = activePlans[currentIndex];
      artifacts[currentIndex] = await buildProviderArtifact(
        providerPlan.provider,
        input.project,
        outputRoot,
        materializedEntry
      );
    }
  });

  await Promise.all(workers);
  return { artifacts: artifacts.filter((artifact): artifact is BuildArtifact => Boolean(artifact)) };
}
