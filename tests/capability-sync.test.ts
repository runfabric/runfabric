import test from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import { RUNTIME_FAMILIES } from "@runfabric/core";
import { capabilityMatrix } from "@runfabric/planner";

const repoRoot = fileURLToPath(new URL("..", import.meta.url));

test("provider capability matrix is synchronized", () => {
  const result = spawnSync("node", ["scripts/sync-provider-capabilities.mjs", "--check"], {
    cwd: repoRoot,
    encoding: "utf8"
  });

  assert.equal(result.status, 0, result.stderr || result.stdout);
  assert.ok((result.stdout || "").includes("capability matrix is in sync"));
});

test("provider runtime capability matrix declares canonical runtime families", () => {
  for (const [provider, capabilities] of Object.entries(capabilityMatrix)) {
    assert.ok(capabilities.supportedRuntimes.length > 0, `${provider} must declare at least one runtime`);
    const unique = new Set(capabilities.supportedRuntimes);
    assert.equal(
      unique.size,
      capabilities.supportedRuntimes.length,
      `${provider} supportedRuntimes must not contain duplicates`
    );
    for (const runtime of capabilities.supportedRuntimes) {
      assert.ok(
        RUNTIME_FAMILIES.includes(runtime),
        `${provider} runtime ${runtime} must be one of ${RUNTIME_FAMILIES.join(", ")}`
      );
    }
  }
});
