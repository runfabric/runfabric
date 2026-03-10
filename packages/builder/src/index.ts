import { createHash } from "node:crypto";
import { copyFile, mkdir, readFile, writeFile } from "node:fs/promises";
import { basename, join, resolve } from "node:path";
import type { BuildArtifact, BuildResult, ProjectConfig } from "@runfabric/core";
import type { PlanningResult } from "@runfabric/planner";

export interface BuildProjectInput {
  planning: PlanningResult;
  project: ProjectConfig;
  projectDir: string;
  outputRoot?: string;
}

interface GeneratedFile {
  path: string;
  bytes: number;
  sha256: string;
  role: "entry-source" | "runtime-wrapper" | "manifest";
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

function createRuntimeWrapperContent(
  provider: string,
  sourceEntryRelativePath: string,
  service: string
): string {
  const importSource = `./${sourceEntryRelativePath}`;

  if (provider === "cloudflare-workers") {
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

  const runtimeName =
    provider === "aws-lambda"
      ? "aws-lambda"
      : provider === "gcp-functions"
        ? "gcp-functions"
        : provider === "azure-functions"
          ? "azure-functions"
          : "generic";

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

function hashContent(content: string): string {
  return createHash("sha256").update(content).digest("hex");
}

async function readFileAsUtf8(filePath: string): Promise<string> {
  return readFile(filePath, "utf8");
}

async function buildProviderArtifact(
  provider: string,
  project: ProjectConfig,
  projectDir: string,
  outputRoot: string
): Promise<BuildArtifact> {
  const providerRoot = join(outputRoot, provider, project.service);
  const sourceDir = join(providerRoot, "src");
  const runtimeDir = join(providerRoot, "runtime");
  const manifestPath = join(providerRoot, "artifact.json");

  const sourceEntryPath = resolve(projectDir, project.entry);
  const sourceEntryName = basename(project.entry);
  const copiedEntryPath = join(sourceDir, sourceEntryName);
  const runtimeFileName = runtimeWrapperFilename(provider);
  const runtimeFilePath = join(runtimeDir, runtimeFileName);

  await mkdir(sourceDir, { recursive: true });
  await mkdir(runtimeDir, { recursive: true });
  await copyFile(sourceEntryPath, copiedEntryPath);

  const relativeSourceFromRuntime = `../src/${sourceEntryName}`;
  const wrapperContent = createRuntimeWrapperContent(provider, relativeSourceFromRuntime, project.service);
  await writeFile(runtimeFilePath, wrapperContent, "utf8");

  const copiedEntryContent = await readFileAsUtf8(copiedEntryPath);
  const generatedFiles: GeneratedFile[] = [
    {
      path: copiedEntryPath,
      bytes: Buffer.byteLength(copiedEntryContent, "utf8"),
      sha256: hashContent(copiedEntryContent),
      role: "entry-source"
    },
    {
      path: runtimeFilePath,
      bytes: Buffer.byteLength(wrapperContent, "utf8"),
      sha256: hashContent(wrapperContent),
      role: "runtime-wrapper"
    }
  ];

  const manifestContent = {
    provider,
    service: project.service,
    runtime: project.runtime,
    entry: project.entry,
    buildVersion: 1,
    generatedAt: new Date().toISOString(),
    files: generatedFiles
  };
  const manifestJson = JSON.stringify(manifestContent, null, 2);
  generatedFiles.push({
    path: manifestPath,
    bytes: Buffer.byteLength(manifestJson, "utf8"),
    sha256: hashContent(manifestJson),
    role: "manifest"
  });

  await writeFile(manifestPath, JSON.stringify({ ...manifestContent, files: generatedFiles }, null, 2), "utf8");

  return {
    provider,
    entry: runtimeFilePath,
    outputPath: manifestPath
  };
}

export async function buildProject(input: BuildProjectInput): Promise<BuildResult> {
  const outputRoot = input.outputRoot || resolve(input.projectDir, ".runfabric", "build");
  const artifacts: BuildArtifact[] = [];

  for (const providerPlan of input.planning.providerPlans) {
    if (providerPlan.errors.length > 0) {
      continue;
    }

    const artifact = await buildProviderArtifact(
      providerPlan.provider,
      input.project,
      input.projectDir,
      outputRoot
    );
    artifacts.push(artifact);
  }

  return { artifacts };
}
