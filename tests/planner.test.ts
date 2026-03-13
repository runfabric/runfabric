import test from "node:test";
import assert from "node:assert/strict";
import {
  AwsQueueFunctionResponseTypeEnum,
  TriggerEnum
} from "../packages/core/src/index.ts";
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
  assert.equal(project.triggers[0].type, TriggerEnum.Http);
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

test("parseProjectConfig resolves env bindings with optional default values", () => {
  const previousService = process.env.RUNFABRIC_TEST_SERVICE_NAME;
  const previousBucket = process.env.RUNFABRIC_TEST_BUCKET_NAME;
  process.env.RUNFABRIC_TEST_SERVICE_NAME = "env-bound-service";
  process.env.RUNFABRIC_TEST_BUCKET_NAME = "env-state-bucket";

  const config = [
    "service: ${env:RUNFABRIC_TEST_SERVICE_NAME}",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "state:",
    "  backend: s3",
    "  s3:",
    "    bucket: ${env:RUNFABRIC_TEST_BUCKET_NAME}",
    "    region: ${env:RUNFABRIC_TEST_AWS_REGION,us-east-1}",
    "",
    "env:",
    "  REGION_NAME: ${env:RUNFABRIC_TEST_AWS_REGION,us-east-1}",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /env",
    ""
  ].join("\n");

  try {
    const project = parseProjectConfig(config);
    assert.equal(project.service, "env-bound-service");
    assert.equal(project.state?.backend, "s3");
    assert.equal(project.state?.s3?.bucket, "env-state-bucket");
    assert.equal(project.state?.s3?.region, "us-east-1");
    assert.equal(project.env?.REGION_NAME, "us-east-1");
  } finally {
    if (previousService === undefined) {
      delete process.env.RUNFABRIC_TEST_SERVICE_NAME;
    } else {
      process.env.RUNFABRIC_TEST_SERVICE_NAME = previousService;
    }
    if (previousBucket === undefined) {
      delete process.env.RUNFABRIC_TEST_BUCKET_NAME;
    } else {
      process.env.RUNFABRIC_TEST_BUCKET_NAME = previousBucket;
    }
  }
});

test("parseProjectConfig reports missing env bindings without defaults", () => {
  const previousMissing = process.env.RUNFABRIC_TEST_MISSING_ENV;
  delete process.env.RUNFABRIC_TEST_MISSING_ENV;

  const config = [
    "service: missing-env",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "env:",
    "  REQUIRED: ${env:RUNFABRIC_TEST_MISSING_ENV}",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /missing-env",
    ""
  ].join("\n");

  try {
    assert.throws(
      () => parseProjectConfig(config),
      /root\.env\.REQUIRED references missing environment variable RUNFABRIC_TEST_MISSING_ENV/
    );
  } finally {
    if (previousMissing !== undefined) {
      process.env.RUNFABRIC_TEST_MISSING_ENV = previousMissing;
    }
  }
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

test("parseProjectConfig parses deploy rollback policy and applies stage overrides", () => {
  const config = [
    "service: deploy-policy",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /deploy-policy",
    "",
    "deploy:",
    "  rollbackOnFailure: false",
    "",
    "stages:",
    "  prod:",
    "    deploy:",
    "      rollbackOnFailure: true",
    ""
  ].join("\n");

  const defaultProject = parseProjectConfig(config);
  assert.equal(defaultProject.deploy?.rollbackOnFailure, false);

  const prodProject = parseProjectConfig(config, { stage: "prod" });
  assert.equal(prodProject.deploy?.rollbackOnFailure, true);
});

test("parseProjectConfig validates deploy rollback policy type", () => {
  const invalidConfig = [
    "service: deploy-policy-invalid",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /deploy-policy-invalid",
    "",
    "deploy:",
    "  rollbackOnFailure: maybe",
    ""
  ].join("\n");

  assert.throws(() => parseProjectConfig(invalidConfig), /deploy\.rollbackOnFailure must be a boolean/);
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

test("parseProjectConfig parses state backend schema", () => {
  const config = [
    "service: state-schema",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "state:",
    "  backend: s3",
    "  keyPrefix: runfabric/custom",
    "  lock:",
    "    enabled: true",
    "    timeoutSeconds: 45",
    "    heartbeatSeconds: 15",
    "    staleAfterSeconds: 90",
    "  s3:",
    "    bucket: deploy-state",
    "    region: us-east-1",
    "    keyPrefix: runfabric/s3-state",
    "    useLockfile: true",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /state",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  assert.equal(project.state?.backend, "s3");
  assert.equal(project.state?.keyPrefix, "runfabric/custom");
  assert.equal(project.state?.lock?.timeoutSeconds, 45);
  assert.equal(project.state?.s3?.bucket, "deploy-state");
  assert.equal(project.state?.s3?.keyPrefix, "runfabric/s3-state");
});

test("parseProjectConfig validates backend-specific state requirements", () => {
  const invalidS3 = [
    "service: invalid-state-s3",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "state:",
    "  backend: s3",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /state",
    ""
  ].join("\n");

  assert.throws(
    () => parseProjectConfig(invalidS3),
    /state\.s3\.bucket is required when state\.backend is s3/
  );
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

test("createPlan supports kubernetes for http trigger", () => {
  const config = [
    "service: k8s-http",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - kubernetes",
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

test("createPlan reports unsupported queue trigger on kubernetes", () => {
  const config = [
    "service: k8s-queue",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - kubernetes",
    "",
    "triggers:",
    "  - type: queue",
    "    queue: jobs",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const plan = createPlan(project);
  assert.equal(plan.ok, false);
  assert.ok(plan.errors.some((value) => value.includes("queue trigger is not supported")));
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

test("parseProjectConfig parses queue/storage trigger fields, aws iam extension, and function env", () => {
  const config = [
    "service: aws-events",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: queue",
    "    queue: arn:aws:sqs:us-east-1:123456789012:jobs",
    "    batchSize: 10",
    "    maximumBatchingWindowSeconds: 5",
    "    enabled: true",
    "    functionResponseType: ReportBatchItemFailures",
    "  - type: storage",
    "    bucket: uploads",
    "    events:",
    "      - s3:ObjectCreated:*",
    "    prefix: incoming/",
    "    suffix: .json",
    "    existingBucket: true",
    "",
    "functions:",
    "  - name: save",
    "    entry: src/save.ts",
    "    env:",
    "      BUCKET: uploads",
    "",
    "extensions:",
    "  aws-lambda:",
    "    stage: prod",
    "    region: us-east-1",
    "    iam:",
    "      role:",
    "        statements:",
    "          - effect: Allow",
    "            actions:",
    "              - s3:PutObject",
    "            resources:",
    "              - arn:aws:s3:::uploads/*",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  const queueTrigger = project.triggers[0];
  const storageTrigger = project.triggers[1];

  assert.equal(queueTrigger.type, "queue");
  assert.equal(queueTrigger.batchSize, 10);
  assert.equal(queueTrigger.maximumBatchingWindowSeconds, 5);
  assert.equal(queueTrigger.enabled, true);
  assert.equal(
    queueTrigger.functionResponseType,
    AwsQueueFunctionResponseTypeEnum.ReportBatchItemFailures
  );

  assert.equal(storageTrigger.type, "storage");
  assert.equal(storageTrigger.bucket, "uploads");
  assert.deepEqual(storageTrigger.events, ["s3:ObjectCreated:*"]);
  assert.equal(storageTrigger.existingBucket, true);
  assert.equal(project.functions?.[0]?.env?.BUCKET, "uploads");

  const awsExt = project.extensions?.["aws-lambda"] as Record<string, unknown> | undefined;
  assert.ok(awsExt);
  const iam = awsExt?.iam as Record<string, unknown>;
  const role = iam.role as Record<string, unknown>;
  const statements = role.statements as Array<Record<string, unknown>>;
  assert.equal(statements[0].effect, "Allow");
});

test("parseProjectConfig rejects invalid aws iam statement schema", () => {
  const invalidConfig = [
    "service: invalid-iam",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /hello",
    "",
    "extensions:",
    "  aws-lambda:",
    "    iam:",
    "      role:",
    "        statements:",
    "          - effect: Maybe",
    "            actions:",
    "              - s3:PutObject",
    "            resources:",
    "              - arn:aws:s3:::uploads/*",
    ""
  ].join("\n");

  assert.throws(() => parseProjectConfig(invalidConfig), /effect must be Allow or Deny/);
});

test("createPlan validates queue/storage required fields", () => {
  const queueMissing = createPlan({
    service: "queue-missing",
    runtime: "nodejs",
    entry: "src/index.ts",
    stage: "default",
    providers: ["aws-lambda"],
    triggers: [{ type: TriggerEnum.Queue }]
  });
  assert.equal(queueMissing.ok, false);
  assert.ok(queueMissing.errors.some((message) => message.includes("queue trigger requires queue")));

  const storageMissing = createPlan({
    service: "storage-missing",
    runtime: "nodejs",
    entry: "src/index.ts",
    stage: "default",
    providers: ["aws-lambda"],
    triggers: [{ type: TriggerEnum.Storage, bucket: "uploads" }]
  });
  assert.equal(storageMissing.ok, false);
  assert.ok(storageMissing.errors.some((message) => message.includes("storage trigger requires events")));
});

test("createPlan reports unsupported storage trigger on cloudflare-workers", () => {
  const plan = createPlan({
    service: "storage-worker",
    runtime: "nodejs",
    entry: "src/worker.ts",
    stage: "default",
    providers: ["cloudflare-workers"],
    triggers: [
      {
        type: TriggerEnum.Storage,
        bucket: "uploads",
        events: ["s3:ObjectCreated:*"]
      }
    ]
  });

  assert.equal(plan.ok, false);
  assert.ok(plan.errors.some((message) => message.includes("storage trigger is not supported")));
});

test("parseProjectConfig parses eventbridge/pubsub/kafka/rabbitmq trigger schemas", () => {
  const config = [
    "service: advanced-triggers",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "  - gcp-functions",
    "",
    "triggers:",
    "  - type: eventbridge",
    "    bus: default",
    "    pattern:",
    "      source:",
    "        - runfabric.test",
    "  - type: pubsub",
    "    topic: projects/test/topics/events",
    "    subscription: projects/test/subscriptions/events-sub",
    "  - type: kafka",
    "    brokers:",
    "      - broker-1:9092",
    "      - broker-2:9092",
    "    topic: stream-events",
    "    groupId: consumer-a",
    "  - type: rabbitmq",
    "    queue: jobs",
    "    exchange: jobs-exchange",
    "    routingKey: jobs.created",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  assert.equal(project.triggers[0].type, TriggerEnum.EventBridge);
  assert.equal(project.triggers[1].type, TriggerEnum.PubSub);
  assert.equal(project.triggers[2].type, TriggerEnum.Kafka);
  assert.equal(project.triggers[3].type, TriggerEnum.RabbitMq);
});

test("parseProjectConfig parses workflows/resources/secrets schema", () => {
  const config = [
    "service: workflow-resources-secrets",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /",
    "",
    "resources:",
    "  queues:",
    "    - name: jobs",
    "      fifo: false",
    "  buckets:",
    "    - name: uploads",
    "  topics:",
    "    - name: events",
    "  databases:",
    "    - name: appdb",
    "      engine: postgres",
    "",
    "secrets:",
    "  DB_PASSWORD: secret://prod/app/db-password",
    "",
    "workflows:",
    "  - name: process-pipeline",
    "    steps:",
    "      - function: ingest",
    "        next: transform",
    "        retry:",
    "          attempts: 3",
    "          backoffSeconds: 5",
    "      - function: transform",
    "        timeoutSeconds: 120",
    ""
  ].join("\n");

  const project = parseProjectConfig(config);
  assert.equal(project.resources?.queues?.[0]?.name, "jobs");
  assert.equal(project.resources?.databases?.[0]?.name, "appdb");
  assert.equal(project.secrets?.DB_PASSWORD, "secret://prod/app/db-password");
  assert.equal(project.workflows?.[0]?.name, "process-pipeline");
  assert.equal(project.workflows?.[0]?.steps.length, 2);
});

test("parseProjectConfig rejects invalid secrets schema value", () => {
  const config = [
    "service: invalid-secrets",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /",
    "",
    "secrets:",
    "  DB_PASSWORD: plaintext-secret",
    ""
  ].join("\n");

  assert.throws(() => parseProjectConfig(config), /secrets\.DB_PASSWORD must use secret:\/\/<ref> format/);
});

test("createPlan validates support for new trigger types with supported non-node runtime", () => {
  const config = [
    "service: non-node-advanced",
    "runtime: python",
    "entry: src/index.py",
    "",
    "providers:",
    "  - aws-lambda",
    "  - gcp-functions",
    "",
    "triggers:",
    "  - type: eventbridge",
    "    pattern:",
    "      source:",
    "        - runfabric.test",
    "  - type: pubsub",
    "    topic: projects/test/topics/events",
    ""
  ].join("\n");

  const plan = createPlan(parseProjectConfig(config));
  assert.equal(plan.ok, false);
  assert.equal(plan.warnings.some((warning) => warning.includes("runtime")), false);
  assert.ok(plan.errors.some((error) => error.includes("gcp-functions: eventbridge trigger is not supported")));
  assert.ok(plan.errors.some((error) => error.includes("aws-lambda: pubsub trigger is not supported")));
  assert.ok(plan.portability.partiallySupportedTriggerTypes.includes("eventbridge"));
  assert.ok(plan.portability.partiallySupportedTriggerTypes.includes("pubsub"));
});

test("createPlan reports provider-specific unsupported runtime errors", () => {
  const config = [
    "service: runtime-mismatch",
    "runtime: python",
    "entry: src/index.py",
    "",
    "providers:",
    "  - cloudflare-workers",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /",
    ""
  ].join("\n");

  const plan = createPlan(parseProjectConfig(config));
  assert.equal(plan.ok, false);
  assert.ok(
    plan.errors.some((error) =>
      error.includes("cloudflare-workers: runtime python is not supported")
    )
  );
});

test("createPlan reports provider-specific unsupported engine runtime mode", () => {
  const config = [
    "service: engine-mode-mismatch",
    "runtime: nodejs",
    "runtimeMode: engine",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - cloudflare-workers",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /",
    ""
  ].join("\n");

  const plan = createPlan(parseProjectConfig(config));
  assert.equal(plan.ok, false);
  assert.ok(
    plan.errors.some((error) =>
      error.includes("cloudflare-workers: runtimeMode engine is not supported for this provider")
    )
  );
});

test("createPlan allows supported provider in engine runtime mode", () => {
  const config = [
    "service: engine-mode-supported",
    "runtime: nodejs",
    "runtimeMode: engine",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /",
    ""
  ].join("\n");

  const plan = createPlan(parseProjectConfig(config));
  assert.equal(
    plan.errors.some((error) => error.includes("runtimeMode engine is not supported for this provider")),
    false
  );
});

test("parseProjectConfig rejects unsupported runtime family", () => {
  const config = [
    "service: invalid-runtime",
    "runtime: ruby",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
    "",
    "triggers:",
    "  - type: http",
    "    method: GET",
    "    path: /",
    ""
  ].join("\n");

  assert.throws(
    () => parseProjectConfig(config),
    /runtime must be one of: nodejs \| python \| go \| java \| rust \| dotnet/
  );
});
