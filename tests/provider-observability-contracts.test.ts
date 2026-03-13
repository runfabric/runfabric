import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {
  clearProviderRegistryCacheForTests,
  createProviderRegistry,
  providerRegistryCacheSnapshotForTests
} from "../apps/cli/src/providers/registry.ts";
import { createVercelProvider } from "../packages/provider-vercel/src/index.ts";

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

test("provider registry rejects unsafe *_TRACES_CMD shell operators", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-observability-unsafe-"));

  await withEnv(
    {
      RUNFABRIC_AWS_TRACES_CMD:
        "printf '%s' '[{\"timestamp\":\"2026-01-01T00:00:00.000Z\",\"message\":\"ok\"}]' && printf '%s' '[]'"
    },
    async () => {
      const providers = createProviderRegistry(projectDir);
      const aws = providers["aws-lambda"];
      await assert.rejects(
        aws.traces?.({ provider: "aws-lambda" }) as Promise<unknown>,
        /unsafe command in RUNFABRIC_AWS_TRACES_CMD/
      );
    }
  );
});

test("provider registry rejects unsafe *_TRACES_CMD background operator", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-observability-unsafe-bg-"));

  await withEnv(
    {
      RUNFABRIC_AWS_TRACES_CMD:
        "printf '%s' '[{\"timestamp\":\"2026-01-01T00:00:00.000Z\",\"message\":\"ok\"}]' & printf '%s' '[]'"
    },
    async () => {
      const providers = createProviderRegistry(projectDir);
      const aws = providers["aws-lambda"];
      await assert.rejects(
        aws.traces?.({ provider: "aws-lambda" }) as Promise<unknown>,
        /unsafe command in RUNFABRIC_AWS_TRACES_CMD/
      );
    }
  );
});

test("provider registry caches loaded provider modules and factories", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-observability-cache-"));
  clearProviderRegistryCacheForTests();

  const initial = providerRegistryCacheSnapshotForTests();
  assert.equal(initial.moduleEntries, 0);
  assert.equal(initial.factoryEntries, 0);

  createProviderRegistry(projectDir, ["aws-lambda", "gcp-functions"]);
  const afterFirstLoad = providerRegistryCacheSnapshotForTests();
  assert.ok(afterFirstLoad.moduleEntries >= 2);
  assert.ok(afterFirstLoad.factoryEntries >= 2);

  createProviderRegistry(projectDir, ["aws-lambda", "gcp-functions"]);
  const afterSecondLoad = providerRegistryCacheSnapshotForTests();
  assert.deepEqual(afterSecondLoad, afterFirstLoad);
});

test("provider-native observability commands execute for vercel when real deploy mode is enabled", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-observability-native-"));
  const vercel = createVercelProvider({ projectDir });

  await withEnv(
    {
      RUNFABRIC_VERCEL_REAL_DEPLOY: "1",
      RUNFABRIC_VERCEL_TRACES_CMD:
        "printf '%s' '[{\"timestamp\":\"2026-01-01T00:00:00.000Z\",\"message\":\"vercel native trace\"}]'",
      RUNFABRIC_VERCEL_METRICS_CMD: "printf '%s' '[{\"name\":\"vercel_requests\",\"value\":7,\"unit\":\"count\"}]'"
    },
    async () => {
      const traces = await vercel.traces?.({ provider: "vercel" });
      const metrics = await vercel.metrics?.({ provider: "vercel" });

      assert.equal(traces?.traces[0]?.message, "vercel native trace");
      assert.equal(metrics?.metrics[0]?.name, "vercel_requests");
      assert.equal(metrics?.metrics[0]?.value, 7);
    }
  );
});

test("provider-native observability commands are gated behind real deploy mode", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-observability-native-gated-"));
  const vercel = createVercelProvider({ projectDir });

  await withEnv(
    {
      RUNFABRIC_VERCEL_TRACES_CMD:
        "printf '%s' '[{\"timestamp\":\"2026-01-01T00:00:00.000Z\",\"message\":\"should not execute\"}]'",
      RUNFABRIC_VERCEL_METRICS_CMD: "printf '%s' '[{\"name\":\"should_not_execute\",\"value\":1}]'"
    },
    async () => {
      const traces = await vercel.traces?.({ provider: "vercel" });
      const metrics = await vercel.metrics?.({ provider: "vercel" });

      assert.deepEqual(traces?.traces, []);
      assert.ok(metrics?.metrics.some((metric) => metric.name === "deploy_total"));
    }
  );
});
