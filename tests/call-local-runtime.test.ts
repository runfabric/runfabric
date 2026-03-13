import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { setTimeout as delay } from "node:timers/promises";
import { loadHandler } from "../apps/cli/src/commands/call-local/runtime.ts";

function bodyFromInvokeResult(value: unknown): string {
  if (!value || typeof value !== "object") {
    return "";
  }
  const body = (value as Record<string, unknown>).body;
  return typeof body === "string" ? body : "";
}

async function writeHandlerModule(path: string, tag: string): Promise<void> {
  await writeFile(
    path,
    [
      "globalThis.__runfabricFreshLoadCount = (globalThis.__runfabricFreshLoadCount || 0) + 1;",
      `export const handler = async () => ({ statusCode: 200, body: String(globalThis.__runfabricFreshLoadCount) + ":${tag}" });`,
      ""
    ].join("\n"),
    "utf8"
  );
}

test("loadHandler fresh mode reuses module cache until file changes", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-call-local-runtime-"));
  const srcDir = join(projectDir, "src");
  const entryPath = join(srcDir, "index.mjs");

  await mkdir(srcDir, { recursive: true });
  await writeHandlerModule(entryPath, "v1");

  const firstHandler = await loadHandler(projectDir, "src/index.mjs", true);
  const secondHandler = await loadHandler(projectDir, "src/index.mjs", true);
  const firstBody = bodyFromInvokeResult(await firstHandler({}));
  const secondBody = bodyFromInvokeResult(await secondHandler({}));

  assert.equal(
    firstBody,
    secondBody,
    "fresh load should reuse the same module when the entry file is unchanged"
  );

  await delay(20);
  await writeHandlerModule(entryPath, "v2");

  const thirdHandler = await loadHandler(projectDir, "src/index.mjs", true);
  const thirdBody = bodyFromInvokeResult(await thirdHandler({}));
  assert.notEqual(
    thirdBody,
    secondBody,
    "fresh load should invalidate cache when the entry file changes"
  );
});

test("loadHandler resolves .tsx entry via built dist artifact", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-call-local-runtime-tsx-"));
  const distSrcDir = join(projectDir, "dist", "src");
  const distEntryPath = join(distSrcDir, "index.js");

  await mkdir(distSrcDir, { recursive: true });
  await writeFile(
    distEntryPath,
    [
      "exports.handler = async () => ({",
      "  statusCode: 200,",
      "  body: 'tsx-dist'",
      "});",
      ""
    ].join("\n"),
    "utf8"
  );

  const handler = await loadHandler(projectDir, "src/index.tsx");
  const responseBody = bodyFromInvokeResult(await handler({}));
  assert.equal(responseBody, "tsx-dist");
});
