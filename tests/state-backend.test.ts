import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { LocalFileStateBackend } from "../packages/core/src/state.ts";

test("local state backend read/write/lock/unlock", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-state-"));
  const backend = new LocalFileStateBackend({ projectDir });
  const address = {
    service: "state-test",
    stage: "default",
    provider: "cloudflare-workers"
  };

  const missing = await backend.read(address);
  assert.equal(missing, null);

  await backend.lock(address);
  await backend.write(address, {
    schemaVersion: 1,
    provider: address.provider,
    service: address.service,
    stage: address.stage,
    endpoint: "https://example.workers.dev",
    updatedAt: new Date().toISOString(),
    details: {
      deploymentId: "abc123"
    }
  });
  await backend.unlock(address);

  const stored = await backend.read(address);
  assert.ok(stored);
  assert.equal(stored?.provider, address.provider);
  assert.equal(stored?.stage, address.stage);
  assert.equal(stored?.endpoint, "https://example.workers.dev");
});
