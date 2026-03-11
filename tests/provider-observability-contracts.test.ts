import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { createProviderRegistry } from "../apps/cli/src/providers/registry.ts";

function withEnv(overrides: Record<string, string>, fn: () => Promise<void>): Promise<void> {
  const previous = new Map<string, string | undefined>();
  for (const [key, value] of Object.entries(overrides)) {
    previous.set(key, process.env[key]);
    process.env[key] = value;
  }

  return fn().finally(() => {
    for (const [key, value] of previous.entries()) {
      if (value === undefined) {
        delete process.env[key];
      } else {
        process.env[key] = value;
      }
    }
  });
}

test("provider registry uses *_TRACES_CMD and *_METRICS_CMD when configured", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-observability-"));

  await withEnv(
    {
      RUNFABRIC_AWS_TRACES_CMD:
        "printf '%s' '[{\"timestamp\":\"2026-01-01T00:00:00.000Z\",\"message\":\"aws trace\",\"deploymentId\":\"d-1\"}]'",
      RUNFABRIC_AWS_METRICS_CMD: "printf '%s' '[{\"name\":\"aws_invocations\",\"value\":3,\"unit\":\"count\"}]'"
    },
    async () => {
      const providers = createProviderRegistry(projectDir);
      const aws = providers["aws-lambda"];
      const traces = await aws.traces?.({ provider: "aws-lambda" });
      const metrics = await aws.metrics?.({ provider: "aws-lambda" });

      assert.ok(traces);
      assert.ok(metrics);
      assert.equal(traces?.traces[0]?.message, "aws trace");
      assert.equal(metrics?.metrics[0]?.name, "aws_invocations");
      assert.equal(metrics?.metrics[0]?.value, 3);
    }
  );
});
