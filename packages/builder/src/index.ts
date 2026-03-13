import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { spawn } from "node:child_process";
import { access, constants, mkdir, readFile, writeFile } from "node:fs/promises";
import { basename, extname, join, resolve } from "node:path";
import { ARTIFACT_MANIFEST_SCHEMA_VERSION, ENGINE_CONTRACT_ABI_VERSION, ENGINE_CONTRACT_API_VERSION, ENGINE_CONTRACT_COMPATIBILITY_POLICY, assertArtifactManifest, type BuildArtifact, type BuildResult, type ProjectConfig, type RuntimeFamily } from "@runfabric/core";
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
  role: "entry-source" | "runtime-wrapper" | "runtime-package" | "manifest";
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

interface RuntimePackagingContext {
  project: ProjectConfig;
  projectDir: string;
  runtimeDir: string;
  copiedEntryPath: string;
  generatedFiles: GeneratedFile[];
}

type RuntimePackagingAdapter = (context: RuntimePackagingContext) => Promise<string>;

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

function isNodeRuntime(runtime: RuntimeFamily): boolean {
  return runtime === "nodejs";
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

function hashContent(content: string | Buffer): string {
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
  const shouldTranspile = isNodeRuntime(project.runtime) && (sourceEntryExt === ".ts" || sourceEntryExt === ".tsx");
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
): Promise<string> {
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

async function runBuildCommand(
  command: string,
  args: string[],
  cwd: string,
  description: string
): Promise<void> {
  await new Promise<void>((resolvePromise, rejectPromise) => {
    const stderrChunks: Buffer[] = [];
    const child = spawn(command, args, { cwd, env: process.env });
    if (child.stderr) {
      child.stderr.on("data", (chunk) => stderrChunks.push(Buffer.from(chunk)));
    }
    child.on("error", (error) => {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") {
        rejectPromise(new Error(`${description} requires "${command}" to be available in PATH`));
        return;
      }
      rejectPromise(error);
    });
    child.on("close", (code) => {
      if (code === 0) {
        resolvePromise();
        return;
      }
      const stderr = Buffer.concat(stderrChunks).toString("utf8").trim();
      rejectPromise(new Error(`${description} failed with exit code ${code ?? 1}${stderr ? `: ${stderr}` : ""}`));
    });
  });
}

async function fileExists(pathValue: string): Promise<boolean> {
  try {
    await access(pathValue, constants.F_OK);
    return true;
  } catch {
    return false;
  }
}

async function addGeneratedRuntimeFile(pathValue: string, generatedFiles: GeneratedFile[]): Promise<void> {
  const content = await readFile(pathValue);
  generatedFiles.push({
    path: pathValue,
    bytes: content.byteLength,
    sha256: hashContent(content),
    role: "runtime-package"
  });
}

async function packagePythonRuntime(
  projectDir: string,
  copiedEntryPath: string,
  runtimeDir: string,
  generatedFiles: GeneratedFile[]
): Promise<string> {
  const requirementsPath = resolve(projectDir, "requirements.txt");
  if (!(await fileExists(requirementsPath))) {
    return copiedEntryPath;
  }

  const pythonRoot = join(runtimeDir, "python");
  const venvDir = join(pythonRoot, "venv");
  const packagesDir = join(pythonRoot, "site-packages");
  const pipExecutable = process.platform === "win32" ? join(venvDir, "Scripts", "pip.exe") : join(venvDir, "bin", "pip");
  await mkdir(pythonRoot, { recursive: true });
  await runBuildCommand("python3", ["-m", "venv", venvDir], projectDir, "python venv setup");
  await runBuildCommand(
    pipExecutable,
    ["install", "-r", requirementsPath, "-t", packagesDir],
    projectDir,
    "python dependency packaging"
  );

  const markerPath = join(pythonRoot, "packaged-with-runfabric.txt");
  const markerContent = [
    "# generated by @runfabric/builder",
    `requirements=${requirementsPath}`,
    `entry=${copiedEntryPath}`
  ].join("\n");
  await writeFile(markerPath, markerContent, "utf8");
  generatedFiles.push({
    path: markerPath,
    bytes: Buffer.byteLength(markerContent, "utf8"),
    sha256: hashContent(markerContent),
    role: "runtime-package"
  });
  return copiedEntryPath;
}

async function packageGoRuntime(
  projectDir: string,
  copiedEntryPath: string,
  runtimeDir: string,
  generatedFiles: GeneratedFile[]
): Promise<string> {
  if (extname(copiedEntryPath).toLowerCase() !== ".go") {
    return copiedEntryPath;
  }
  const goRoot = join(runtimeDir, "go");
  const binaryPath = join(goRoot, "bootstrap");
  await mkdir(goRoot, { recursive: true });
  await runBuildCommand("go", ["build", "-o", binaryPath, copiedEntryPath], projectDir, "go build");
  await addGeneratedRuntimeFile(binaryPath, generatedFiles);
  return binaryPath;
}

async function packageJavaRuntime(
  projectDir: string,
  copiedEntryPath: string,
  runtimeDir: string,
  generatedFiles: GeneratedFile[]
): Promise<string> {
  const extension = extname(copiedEntryPath).toLowerCase();
  if (extension === ".jar" || extension !== ".java") {
    return copiedEntryPath;
  }
  const javaRoot = join(runtimeDir, "java");
  const classesDir = join(javaRoot, "classes");
  const jarPath = join(javaRoot, "app.jar");
  await mkdir(classesDir, { recursive: true });
  await runBuildCommand("javac", ["-d", classesDir, copiedEntryPath], projectDir, "java compilation");
  await runBuildCommand("jar", ["--create", "--file", jarPath, "-C", classesDir, "."], projectDir, "java jar packaging");
  await addGeneratedRuntimeFile(jarPath, generatedFiles);
  return jarPath;
}

async function packageRustRuntime(
  projectDir: string,
  copiedEntryPath: string,
  runtimeDir: string,
  generatedFiles: GeneratedFile[]
): Promise<string> {
  if (extname(copiedEntryPath).toLowerCase() !== ".rs") {
    return copiedEntryPath;
  }
  const rustRoot = join(runtimeDir, "rust");
  const binaryPath = join(rustRoot, "bootstrap");
  await mkdir(rustRoot, { recursive: true });
  await runBuildCommand("rustc", [copiedEntryPath, "-O", "-o", binaryPath], projectDir, "rust build");
  await addGeneratedRuntimeFile(binaryPath, generatedFiles);
  return binaryPath;
}

async function packageDotnetRuntime(
  project: ProjectConfig,
  projectDir: string,
  copiedEntryPath: string,
  runtimeDir: string,
  generatedFiles: GeneratedFile[]
): Promise<string> {
  if (extname(copiedEntryPath).toLowerCase() === ".dll") {
    return copiedEntryPath;
  }

  const dotnetRoot = join(runtimeDir, "dotnet");
  const publishDir = join(dotnetRoot, "publish");
  const projectTarget = extname(copiedEntryPath).toLowerCase() === ".csproj" ? copiedEntryPath : projectDir;
  await mkdir(dotnetRoot, { recursive: true });
  await runBuildCommand("dotnet", ["publish", projectTarget, "-c", "Release", "-o", publishDir], projectDir, "dotnet publish");
  const serviceDll = join(publishDir, `${project.service}.dll`);
  if (await fileExists(serviceDll)) {
    await addGeneratedRuntimeFile(serviceDll, generatedFiles);
    return serviceDll;
  }

  const markerPath = join(dotnetRoot, "publish-manifest.txt");
  const markerContent = [
    "# generated by @runfabric/builder",
    `publishDir=${publishDir}`,
    `entry=${copiedEntryPath}`
  ].join("\n");
  await writeFile(markerPath, markerContent, "utf8");
  generatedFiles.push({
    path: markerPath,
    bytes: Buffer.byteLength(markerContent, "utf8"),
    sha256: hashContent(markerContent),
    role: "runtime-package"
  });
  return publishDir;
}

async function addRuntimePackageFile(
  context: RuntimePackagingContext
): Promise<string> {
  const runtimePackagingAdapters: Record<Exclude<RuntimeFamily, "nodejs">, RuntimePackagingAdapter> = {
    python: ({ projectDir, copiedEntryPath, runtimeDir, generatedFiles }) =>
      packagePythonRuntime(projectDir, copiedEntryPath, runtimeDir, generatedFiles),
    go: ({ projectDir, copiedEntryPath, runtimeDir, generatedFiles }) =>
      packageGoRuntime(projectDir, copiedEntryPath, runtimeDir, generatedFiles),
    java: ({ projectDir, copiedEntryPath, runtimeDir, generatedFiles }) =>
      packageJavaRuntime(projectDir, copiedEntryPath, runtimeDir, generatedFiles),
    rust: ({ projectDir, copiedEntryPath, runtimeDir, generatedFiles }) =>
      packageRustRuntime(projectDir, copiedEntryPath, runtimeDir, generatedFiles),
    dotnet: ({ project, projectDir, copiedEntryPath, runtimeDir, generatedFiles }) =>
      packageDotnetRuntime(project, projectDir, copiedEntryPath, runtimeDir, generatedFiles)
  };

  if (context.project.runtime === "nodejs") {
    return context.copiedEntryPath;
  }
  return runtimePackagingAdapters[context.project.runtime](context);
}

async function writeArtifactManifest(
  provider: string,
  project: ProjectConfig,
  manifestPath: string,
  generatedFiles: GeneratedFile[]
): Promise<void> {
  const files = [...generatedFiles];
  const manifestContent = {
    schemaVersion: ARTIFACT_MANIFEST_SCHEMA_VERSION,
    provider,
    service: project.service,
    runtimeFamily: project.runtime,
    runtimeMode: project.runtimeMode || "native-compat",
    source: {
      entry: project.entry
    },
    engineContract: {
      apiVersion: ENGINE_CONTRACT_API_VERSION,
      abiVersion: ENGINE_CONTRACT_ABI_VERSION,
      compatibilityPolicy: ENGINE_CONTRACT_COMPATIBILITY_POLICY
    },
    build: {
      manifestVersion: ARTIFACT_MANIFEST_SCHEMA_VERSION,
      generatedAt: new Date().toISOString()
    },
    files
  };
  assertArtifactManifest(manifestContent);
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
  projectDir: string,
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
  const runtimeEntry = isNodeRuntime(project.runtime)
    ? await addRuntimeWrapperFile(provider, project, runtimeDir, sourceEntry.copiedEntryName, generatedFiles)
    : await addRuntimePackageFile({
        project,
        projectDir,
        runtimeDir,
        copiedEntryPath,
        generatedFiles
      });
  await writeArtifactManifest(provider, project, manifestPath, generatedFiles);

  return {
    provider,
    entry: runtimeEntry,
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
        input.projectDir,
        outputRoot,
        materializedEntry
      );
    }
  });

  await Promise.all(workers);
  return { artifacts: artifacts.filter((artifact): artifact is BuildArtifact => Boolean(artifact)) };
}
