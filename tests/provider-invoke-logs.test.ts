import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import type { ProjectConfig } from "@runfabric/core";
import {
  buildProviderMetricsFromLocalArtifacts,
  buildProviderTracesFromLocalArtifacts,
  readDeploymentReceipt,
  TriggerEnum
} from "@runfabric/core";
import { createAwsLambdaProvider } from "../packages/provider-aws-lambda/src/index.ts";

function createProject(): ProjectConfig {
  return {
    service: "invoke-logs-http",
    runtime: "nodejs",
    entry: "src/index.ts",
    providers: ["aws-lambda"],
    triggers: [{ type: TriggerEnum.Http, method: "GET", path: "/hello" }]
  };
}

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

test("provider invoke/logs read from deployment artifacts", async () => {
  await withEnv(
    {
      AWS_ACCESS_KEY_ID: "test",
      AWS_SECRET_ACCESS_KEY: "test",
      AWS_REGION: "us-east-1"
    },
    async () => {
      const projectDir = await mkdtemp(join(tmpdir(), "runfabric-provider-ops-"));
      const provider = createAwsLambdaProvider({ projectDir });
      const project = createProject();

      const invokeBeforeDeploy = await provider.invoke?.({
        provider: "aws-lambda",
        payload: JSON.stringify({ ping: true })
      });
      assert.equal(invokeBeforeDeploy?.statusCode, 404);

      const deployPlan = await provider.planDeploy(project, {
        provider: "aws-lambda",
        entry: "src/index.ts",
        outputPath: "manifest.json"
      });
      await provider.deploy(project, deployPlan);

      const invokeAfterDeploy = await provider.invoke?.({
        provider: "aws-lambda",
        payload: JSON.stringify({ ping: "after-deploy" })
      });
      assert.ok(invokeAfterDeploy?.correlation?.invokeId);
      assert.ok(invokeAfterDeploy?.correlation?.deploymentId);

      const logs = await provider.logs?.({ provider: "aws-lambda" });
      assert.ok((logs?.lines.length || 0) > 0);
      assert.ok(logs?.lines.some((line) => line.includes("deploy deploymentId=")));
      assert.ok(logs?.lines.some((line) => line.includes("invokeId=")));

      const receipt = await readDeploymentReceipt(projectDir, "aws-lambda");
      assert.ok(receipt?.correlation?.deployTraceId);
      assert.ok(receipt?.correlation?.latestInvokeId);

      const traces = await buildProviderTracesFromLocalArtifacts(projectDir, "aws-lambda", {
        provider: "aws-lambda"
      });
      assert.ok(traces.traces.length > 0);
      assert.ok(traces.traces.some((trace) => typeof trace.deploymentId === "string"));
      assert.ok(traces.traces.some((trace) => typeof trace.invokeId === "string"));

      const metrics = await buildProviderMetricsFromLocalArtifacts(projectDir, "aws-lambda", {
        provider: "aws-lambda"
      });
      assert.ok(metrics.metrics.some((metric) => metric.name === "invoke_total"));
      assert.ok(metrics.metrics.some((metric) => metric.name === "deploy_total"));
    }
  );
});
