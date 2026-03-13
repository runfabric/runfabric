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

test("provider engine runtime feasibility matrix is explicit and consistent", () => {
  const engineModes = new Set<string>();
  for (const [provider, capabilities] of Object.entries(capabilityMatrix)) {
    engineModes.add(capabilities.engineRuntime);
    assert.ok(
      ["custom-runtime", "container", "unsupported"].includes(capabilities.engineRuntime),
      `${provider} engineRuntime must be custom-runtime, container, or unsupported`
    );
    if (capabilities.engineRuntime === "custom-runtime") {
      assert.equal(capabilities.customRuntime, true, `${provider} custom-runtime engine mode requires customRuntime=true`);
    }
    if (capabilities.engineRuntime === "container") {
      assert.equal(capabilities.containerImage, true, `${provider} container engine mode requires containerImage=true`);
    }
    if (capabilities.engineRuntime === "unsupported") {
      assert.equal(
        capabilities.customRuntime || capabilities.containerImage,
        false,
        `${provider} unsupported engine mode should not expose customRuntime/containerImage support`
      );
    }
  }
  assert.ok(engineModes.has("custom-runtime"), "at least one provider should support engine mode via custom runtime");
  assert.ok(engineModes.has("container"), "at least one provider should support engine mode via container runtime");
  assert.ok(engineModes.has("unsupported"), "at least one provider should be explicitly unsupported in engine mode");
});
