import test from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { spawnSync } from "node:child_process";

const repoRoot = fileURLToPath(new URL("..", import.meta.url));

test("provider capability matrix is synchronized", () => {
  const result = spawnSync("node", ["scripts/sync-provider-capabilities.mjs", "--check"], {
    cwd: repoRoot,
    encoding: "utf8"
  });

  assert.equal(result.status, 0, result.stderr || result.stdout);
  assert.ok((result.stdout || "").includes("capability matrix is in sync"));
});

