import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { parseProjectConfig } from "../packages/planner/src/parse-config.ts";
import { createPlan } from "../packages/planner/src/planner.ts";
import { buildProject } from "../packages/builder/src/index.ts";

function sha256(input: string): string {
  return createHash("sha256").update(input).digest("hex");
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
  const manifestPath = build.artifacts[0].outputPath;
  const manifestRaw = await readFile(manifestPath, "utf8");
  const manifest = JSON.parse(manifestRaw) as {
    files: Array<{ path: string; bytes: number; sha256: string; role: string }>;
  };

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
