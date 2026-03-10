import test from "node:test";
import assert from "node:assert/strict";
import { parseProjectConfig } from "../packages/planner/src/parse-config.ts";
import { createPlan } from "../packages/planner/src/planner.ts";

test("parseProjectConfig parses providers and triggers", () => {
  const config = [
    "service: hello-http",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "  - cloudflare-workers",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /hello",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  assert.equal(project.service, "hello-http");
  assert.equal(project.providers.length, 2);
  assert.equal(project.triggers[0].type, "http");
});

test("parseProjectConfig validates schema and parses extensions scalars", () => {
  const config = [
    "service: schema-check",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /schema",
    "",
    "resources:",
    "  memory: 256",
    "  timeout: 30",
    "",
    "extensions:",
    "  aws-lambda:",
    "    stage: prod",
    "    region: ap-southeast-1",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  assert.equal(project.resources?.memory, 256);
  assert.equal(project.resources?.timeout, 30);
  assert.equal(project.extensions?.["aws-lambda"]?.stage, "prod");
  assert.equal(project.extensions?.["aws-lambda"]?.region, "ap-southeast-1");
});

test("parseProjectConfig rejects invalid providers schema", () => {
  const invalidConfig = [
    "service: invalid-providers",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  aws-lambda: true",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /invalid",
    ""
  ].join("\n");

  assert.throws(() => parseProjectConfig(invalidConfig), /providers must be an array/);
});

test("parseProjectConfig applies stage default and selected stage overrides", () => {
  const config = [
    "service: stage-aware",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /base",
    "",
    "env:",
    "  SHARED_ENV: base",
    "",
    "params:",
    "  region: us-east-1",
    "",
    "extensions:",
    "  aws-lambda:",
    "    stage: base",
    "",
    "stages:",
    "  default:",
    "    env:",
    "      SHARED_ENV: default",
    "      DEFAULT_ONLY: yes",
    "  prod:",
    "    params:",
    "      region: ap-southeast-1",
    "    extensions:",
    "      aws-lambda:",
    "        stage: prod",
    "    triggers:",
    "      - type: http",
    "        method: GET",
    "        path: /prod",
    ""
  ].join("\n");

  const project = parseProjectConfig(config, { stage: "prod" });
  assert.equal(project.stage, "prod");
  assert.equal(project.env?.SHARED_ENV, "default");
  assert.equal(project.env?.DEFAULT_ONLY, "yes");
  assert.equal(project.params?.region, "ap-southeast-1");
  assert.equal(project.extensions?.["aws-lambda"]?.stage, "prod");
  assert.equal(project.triggers[0].path, "/prod");
});

test("parseProjectConfig validates typed provider extensions", () => {
  const invalidConfig = [
    "service: invalid-extensions",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - cloudflare-workers",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /invalid",
    "",
    "extensions:",
    "  cloudflare-workers:",
    "    scriptName: 123",
    ""
  ].join("\n");

  assert.throws(() => parseProjectConfig(invalidConfig), /extensions\.cloudflare-workers\.scriptName must be string/);
});

test("parseProjectConfig parses hooks and function entries", () => {
  const config = [
    "service: hooks-functions",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "hooks:",
    "  - ./hooks/build.mjs",
    "",
    "functions:",
    "  - name: api",
    "    entry: src/api.ts",
    "    runtime: nodejs",
    "    triggers:",
    "      - type: http",
    "        method: GET",
    "        path: /api",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  assert.equal(project.hooks?.[0], "./hooks/build.mjs");
  assert.equal(project.functions?.[0]?.name, "api");
  assert.equal(project.functions?.[0]?.entry, "src/api.ts");
});

test("createPlan reports unsupported queue trigger on cloudflare-workers", () => {
  const config = [
    "service: queue-worker",
    "runtime: nodejs",
    "entry: src/worker.ts",
    "",
    "providers:",
    "  - cloudflare-workers",
    "",
    "triggers:",
    "  - type: queue",
    "    queue: emails",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const plan = createPlan(project);
  assert.equal(plan.ok, false);
  assert.ok(plan.errors.some((value) => value.includes("queue trigger is not supported")));
});

test("createPlan supports gcp and azure providers for http trigger", () => {
  const config = [
    "service: enterprise-http",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - gcp-functions",
    "  - azure-functions",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /health",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const plan = createPlan(project);
  assert.equal(plan.ok, true);
  assert.equal(plan.errors.length, 0);
});

test("createPlan supports alibaba-fc, digitalocean-functions, and fly-machines for http trigger", () => {
  const config = [
    "service: global-http",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - alibaba-fc",
    "  - digitalocean-functions",
    "  - fly-machines",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /health",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const plan = createPlan(project);
  assert.equal(plan.ok, true);
  assert.equal(plan.errors.length, 0);
});

test("createPlan reports unsupported cron trigger on fly-machines", () => {
  const config = [
    "service: cron-service",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - fly-machines",
    "",
    "triggers:",
    "  - type: cron",
    "    schedule: */5 * * * *",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const plan = createPlan(project);
  assert.equal(plan.ok, false);
  assert.ok(plan.errors.some((value) => value.includes("cron trigger is not supported")));
});

test("createPlan supports ibm-openwhisk for http trigger", () => {
  const config = [
    "service: ibm-http",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - ibm-openwhisk",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /status",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const plan = createPlan(project);
  assert.equal(plan.ok, true);
  assert.equal(plan.errors.length, 0);
});

test("createPlan adds portability warning for partially supported triggers", () => {
  const config = [
    "service: portability-check",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "  - cloudflare-workers",
    "",
    "triggers:",
    "  - type: queue",
    "    queue: jobs",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const plan = createPlan(project);
  assert.equal(plan.ok, false);
  assert.ok(plan.warnings.some((value) => value.includes("partial portability")));
  assert.ok(plan.portability.partiallySupportedTriggerTypes.includes("queue"));
});
