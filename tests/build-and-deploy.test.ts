import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, writeFile, readFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { parseProjectConfig } from "../packages/planner/src/parse-config.ts";
import { createPlan } from "../packages/planner/src/planner.ts";
import { buildProject } from "../packages/builder/src/index.ts";
import { createAwsLambdaProvider } from "../packages/provider-aws-lambda/src/index.ts";

test("build + aws deploy writes deployment receipt", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-phase1-"));
  await writeFile(join(projectDir, "src.ts"), "export const handler = async () => ({ status: 200 });", "utf8");

  const config = [
    "service: hello-http",
    "runtime: nodejs",
    "entry: src.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /hello",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const planning = createPlan(project);
  assert.equal(planning.ok, true);

  const build = await buildProject({
    planning,
    project,
    projectDir
  });
  assert.equal(build.artifacts.length, 1);
  assert.equal(build.artifacts[0].provider, "aws-lambda");

  const provider = createAwsLambdaProvider({ projectDir });
  const deployPlan = await provider.planDeploy(project, build.artifacts[0]);
  const deployResult = await provider.deploy(project, deployPlan);
  assert.equal(deployResult.provider, "aws-lambda");
  assert.ok(deployResult.endpoint);

  const receipt = await readFile(join(projectDir, ".runfabric", "deploy", "aws-lambda", "deployment.json"), "utf8");
  assert.ok(receipt.includes("aws-lambda"));
});
