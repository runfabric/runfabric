import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { resolveProjectDir } from "../apps/cli/src/utils/resolve-project.ts";

test("resolveProjectDir uses start directory when default runfabric.yml exists", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-resolve-default-"));
  await writeFile(join(projectDir, "runfabric.yml"), "service: test\nruntime: nodejs\nentry: src/index.ts\nproviders:\n  - aws-lambda\ntriggers:\n  - type: http\n", "utf8");

  const resolved = await resolveProjectDir(projectDir);
  assert.equal(resolved, projectDir);
});

test("resolveProjectDir returns config directory when config path is provided", async () => {
  const rootDir = await mkdtemp(join(tmpdir(), "runfabric-resolve-config-"));
  const nestedDir = join(rootDir, "examples", "hello-http");
  await mkdir(nestedDir, { recursive: true });
  await writeFile(join(nestedDir, "runfabric.yml"), "service: test\nruntime: nodejs\nentry: src/index.ts\nproviders:\n  - aws-lambda\ntriggers:\n  - type: http\n", "utf8");

  const resolved = await resolveProjectDir(rootDir, "examples/hello-http/runfabric.yml");
  assert.equal(resolved, nestedDir);
});
