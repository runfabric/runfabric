import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { spawnSync } from "node:child_process";
import { ARTIFACT_MANIFEST_SCHEMA_VERSION, validateArtifactManifest } from "../packages/core/src/index.ts";
import { parseProjectConfig } from "../packages/planner/src/parse-config.ts";
import { createPlan } from "../packages/planner/src/planner.ts";
import { buildProject } from "../packages/builder/src/index.ts";

function sha256(input: string): string {
  return createHash("sha256").update(input).digest("hex");
}

function hasCommand(command: string, args: string[] = ["--version"]): boolean {
  const result = spawnSync(command, args, { encoding: "utf8" });
  return result.status === 0;
}

async function readManifest(path: string): Promise<Record<string, unknown>> {
  return JSON.parse(await readFile(path, "utf8")) as Record<string, unknown>;
}

function assertManifestV2(manifest: unknown): void {
  assert.deepEqual(validateArtifactManifest(manifest), []);
  const record = manifest as Record<string, unknown>;
  assert.equal(record.schemaVersion, ARTIFACT_MANIFEST_SCHEMA_VERSION);
}

test("artifact manifest does not self-reference and file hashes match content", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-builder-manifest-"));
  await writeFile(join(projectDir, "src.ts"), "export const handler = async () => ({ statusCode: 200 });", "utf8");

  const config = [
    "service: manifest-check",
    "runtime: nodejs",
    "entry: src.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /health",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const planning = createPlan(project);
  const build = await buildProject({
    planning,
    project,
    projectDir
  });

  assert.equal(build.artifacts.length, 1);
  const manifest = (await readManifest(build.artifacts[0].outputPath)) as {
    schemaVersion: number;
    files: Array<{ path: string; bytes: number; sha256: string; role: string }>;
  };
  assertManifestV2(manifest);

  assert.equal(
    manifest.files.some((file) => file.role === "manifest"),
    false,
    "artifact manifest should not include a self-referential manifest entry"
  );

  for (const file of manifest.files) {
    const content = await readFile(file.path, "utf8");
    assert.equal(file.bytes, Buffer.byteLength(content, "utf8"));
    assert.equal(file.sha256, sha256(content));
  }
});

test("python runtime build keeps source entry without node runtime wrapper", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-builder-python-"));
  await writeFile(
    join(projectDir, "src.py"),
    "def handler(event, context):\n    return {\"statusCode\": 200, \"body\": \"ok\"}\n",
    "utf8"
  );

  const config = [
    "service: py-build-check",
    "runtime: python",
    "entry: src.py",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /health",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const planning = createPlan(project);
  const build = await buildProject({ planning, project, projectDir });
  assert.equal(build.artifacts.length, 1);
  assert.ok(build.artifacts[0].entry.endsWith("/src/src.py"));

  const manifest = (await readManifest(build.artifacts[0].outputPath)) as {
    schemaVersion: number;
    files: Array<{ role: string }>;
  };
  assertManifestV2(manifest);
  assert.equal(
    manifest.files.some((file) => file.role === "runtime-wrapper"),
    false
  );
});

test("nodejs runtime build generates runtime wrapper entry", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-builder-node-wrapper-"));
  await writeFile(
    join(projectDir, "src.ts"),
    "export const handler = async () => ({ statusCode: 200, body: \"ok\" });\n",
    "utf8"
  );

  const config = [
    "service: node-wrapper-check",
    "runtime: nodejs",
    "entry: src.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /health",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const planning = createPlan(project);
  const build = await buildProject({ planning, project, projectDir });
  assert.equal(build.artifacts.length, 1);
  assert.ok(build.artifacts[0].entry.endsWith("/runtime/lambda-handler.mjs"));

  const manifest = (await readManifest(build.artifacts[0].outputPath)) as {
    schemaVersion: number;
    files: Array<{ role: string }>;
  };
  assertManifestV2(manifest);
  assert.equal(
    manifest.files.some((file) => file.role === "runtime-wrapper"),
    true
  );
});

test("artifact manifest records engine runtime mode when configured", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-builder-node-engine-mode-"));
  await writeFile(
    join(projectDir, "src.ts"),
    "export const handler = async () => ({ statusCode: 200, body: \"ok\" });\n",
    "utf8"
  );

  const config = [
    "service: node-engine-mode-check",
    "runtime: nodejs",
    "runtimeMode: engine",
    "entry: src.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /health",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const planning = createPlan(project);
  const build = await buildProject({ planning, project, projectDir });
  assert.equal(build.artifacts.length, 1);

  const manifest = (await readManifest(build.artifacts[0].outputPath)) as Record<string, unknown>;
  assertManifestV2(manifest);
  assert.equal(manifest.runtimeMode, "engine");
  assert.equal(manifest.runtimeFamily, "nodejs");
});

test("python runtime packages dependencies when requirements.txt is present", async (t) => {
  if (!hasCommand("python3")) {
    t.skip("python3 is required for python packaging test");
    return;
  }

  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-builder-python-reqs-"));
  await writeFile(
    join(projectDir, "src.py"),
    "def handler(event, context):\n    return {\"statusCode\": 200, \"body\": \"ok\"}\n",
    "utf8"
  );
  await writeFile(join(projectDir, "requirements.txt"), "", "utf8");

  const config = [
    "service: py-requirements-build-check",
    "runtime: python",
    "entry: src.py",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /health",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const planning = createPlan(project);
  const build = await buildProject({ planning, project, projectDir });
  assert.equal(build.artifacts.length, 1);
  assert.ok(build.artifacts[0].entry.endsWith("/src/src.py"));

  const manifest = (await readManifest(build.artifacts[0].outputPath)) as {
    schemaVersion: number;
    files: Array<{ path: string; role: string }>;
  };
  assertManifestV2(manifest);
  assert.equal(
    manifest.files.some((file) => file.role === "runtime-package"),
    true
  );
  assert.equal(
    manifest.files.some((file) => file.path.endsWith("/runtime/python/packaged-with-runfabric.txt")),
    true
  );
});

test("go runtime build emits compiled runtime binary", async (t) => {
  if (!hasCommand("go")) {
    t.skip("go is required for go packaging test");
    return;
  }

  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-builder-go-"));
  await writeFile(
    join(projectDir, "src.go"),
    [
      "package main",
      "",
      "func main() {}"
    ].join("\n"),
    "utf8"
  );

  const config = [
    "service: go-build-check",
    "runtime: go",
    "entry: src.go",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /health",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const planning = createPlan(project);
  const build = await buildProject({ planning, project, projectDir });
  assert.equal(build.artifacts.length, 1);
  assert.ok(build.artifacts[0].entry.endsWith("/runtime/go/bootstrap"));

  const manifest = (await readManifest(build.artifacts[0].outputPath)) as {
    schemaVersion: number;
    files: Array<{ path: string; role: string }>;
  };
  assertManifestV2(manifest);
  assert.equal(
    manifest.files.some((file) => file.role === "runtime-package"),
    true
  );
  assert.equal(
    manifest.files.some((file) => file.path.endsWith("/runtime/go/bootstrap")),
    true
  );
});

test("java runtime build emits packaged jar artifact", async (t) => {
  if (!hasCommand("javac") || !hasCommand("jar", ["--help"])) {
    t.skip("javac and jar are required for java packaging test");
    return;
  }

  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-builder-java-"));
  await writeFile(
    join(projectDir, "Main.java"),
    [
      "public class Main {",
      "  public static void main(String[] args) {",
      "    System.out.println(\"ok\");",
      "  }",
      "}",
      ""
    ].join("\n"),
    "utf8"
  );

  const config = [
    "service: java-build-check",
    "runtime: java",
    "entry: Main.java",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /health",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const planning = createPlan(project);
  const build = await buildProject({ planning, project, projectDir });
  assert.equal(build.artifacts.length, 1);
  assert.ok(build.artifacts[0].entry.endsWith("/runtime/java/app.jar"));

  const manifest = (await readManifest(build.artifacts[0].outputPath)) as {
    schemaVersion: number;
    files: Array<{ path: string; role: string }>;
  };
  assertManifestV2(manifest);
  assert.equal(
    manifest.files.some((file) => file.role === "runtime-package"),
    true
  );
  assert.equal(
    manifest.files.some((file) => file.path.endsWith("/runtime/java/app.jar")),
    true
  );
});

test("rust runtime build emits compiled runtime binary", async (t) => {
  if (!hasCommand("rustc")) {
    t.skip("rustc is required for rust packaging test");
    return;
  }

  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-builder-rust-"));
  await writeFile(
    join(projectDir, "main.rs"),
    [
      "fn main() {",
      "  println!(\"ok\");",
      "}",
      ""
    ].join("\n"),
    "utf8"
  );

  const config = [
    "service: rust-build-check",
    "runtime: rust",
    "entry: main.rs",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /health",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const planning = createPlan(project);
  const build = await buildProject({ planning, project, projectDir });
  assert.equal(build.artifacts.length, 1);
  assert.ok(build.artifacts[0].entry.endsWith("/runtime/rust/bootstrap"));

  const manifest = (await readManifest(build.artifacts[0].outputPath)) as {
    schemaVersion: number;
    files: Array<{ path: string; role: string }>;
  };
  assertManifestV2(manifest);
  assert.equal(
    manifest.files.some((file) => file.role === "runtime-package"),
    true
  );
  assert.equal(
    manifest.files.some((file) => file.path.endsWith("/runtime/rust/bootstrap")),
    true
  );
});

test("dotnet runtime build publishes runtime output", async (t) => {
  if (!hasCommand("dotnet")) {
    t.skip("dotnet is required for dotnet packaging test");
    return;
  }

  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-builder-dotnet-"));
  await writeFile(
    join(projectDir, "app.csproj"),
    [
      "<Project Sdk=\"Microsoft.NET.Sdk\">",
      "  <PropertyGroup>",
      "    <OutputType>Exe</OutputType>",
      "    <TargetFramework>net8.0</TargetFramework>",
      "    <ImplicitUsings>enable</ImplicitUsings>",
      "    <Nullable>enable</Nullable>",
      "  </PropertyGroup>",
      "</Project>",
      ""
    ].join("\n"),
    "utf8"
  );
  await writeFile(
    join(projectDir, "Program.cs"),
    [
      "Console.WriteLine(\"ok\");",
      ""
    ].join("\n"),
    "utf8"
  );

  const config = [
    "service: dotnet-build-check",
    "runtime: dotnet",
    "entry: app.csproj",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /health",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const planning = createPlan(project);
  const build = await buildProject({ planning, project, projectDir });
  assert.equal(build.artifacts.length, 1);
  assert.ok(build.artifacts[0].entry.includes("/runtime/dotnet/publish"));

  const manifest = (await readManifest(build.artifacts[0].outputPath)) as {
    schemaVersion: number;
    files: Array<{ path: string; role: string }>;
  };
  assertManifestV2(manifest);
  assert.equal(
    manifest.files.some((file) => file.role === "runtime-package"),
    true
  );
  assert.equal(
    manifest.files.some((file) => file.path.endsWith("/runtime/dotnet/publish-manifest.txt")),
    true
  );
});
