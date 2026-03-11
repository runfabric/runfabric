import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import type { ProjectConfig } from "@runfabric/core";
import {
  AwsIamEffectEnum,
  AwsQueueFunctionResponseTypeEnum,
  TriggerEnum
} from "@runfabric/core";
import { createAwsLambdaProvider } from "../packages/provider-aws-lambda/src/index.ts";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
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

test("aws provider wires queue/storage/iam/env payloads into deploy command and receipt", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-aws-wiring-"));
  await mkdir(join(projectDir, "src"), { recursive: true });
  await writeFile(join(projectDir, "src", "index.ts"), "export const handler = async () => ({ statusCode: 200, body: 'ok' });\n", "utf8");

  const deployScriptPath = join(projectDir, "echo-deploy.js");
  await writeFile(
    deployScriptPath,
    [
      "console.log(JSON.stringify({",
      "  functionUrl: 'https://example.lambda-url.us-east-1.on.aws/',",
      "  queueEventSources: process.env.RUNFABRIC_AWS_QUEUE_EVENT_SOURCES_JSON,",
      "  storageEvents: process.env.RUNFABRIC_AWS_STORAGE_EVENTS_JSON,",
      "  eventBridgeRules: process.env.RUNFABRIC_AWS_EVENTBRIDGE_RULES_JSON,",
      "  iamRoleStatements: process.env.RUNFABRIC_AWS_IAM_ROLE_STATEMENTS_JSON,",
      "  functionEnv: process.env.RUNFABRIC_FUNCTION_ENV_JSON,",
      "  resourceAddresses: process.env.RUNFABRIC_AWS_RESOURCE_ADDRESSES_JSON,",
      "  workflowAddresses: process.env.RUNFABRIC_AWS_WORKFLOW_ADDRESSES_JSON,",
      "  secretReferences: process.env.RUNFABRIC_AWS_SECRET_REFERENCES_JSON",
      "}));",
      ""
    ].join("\n"),
    "utf8"
  );

  const project: ProjectConfig = {
    service: "aws-wiring",
    runtime: "nodejs",
    entry: "src/index.ts",
    providers: ["aws-lambda"],
    stage: "dev",
    triggers: [
      {
        type: TriggerEnum.Queue,
        queue: "arn:aws:sqs:us-east-1:123456789012:jobs",
        batchSize: 10,
        functionResponseType: AwsQueueFunctionResponseTypeEnum.ReportBatchItemFailures
      },
      {
        type: TriggerEnum.Storage,
        bucket: "uploads",
        events: ["s3:ObjectCreated:*"],
        prefix: "incoming/",
        suffix: ".json"
      },
      {
        type: TriggerEnum.EventBridge,
        bus: "default",
        pattern: {
          source: ["runfabric.test"]
        }
      }
    ],
    env: {
      BUCKET: "uploads"
    },
    secrets: {
      DB_PASSWORD: "secret://prod/db/password"
    },
    workflows: [
      {
        name: "media-pipeline",
        steps: [{ function: "save", timeoutSeconds: 30 }]
      }
    ],
    resources: {
      queues: [{ name: "jobs" }],
      buckets: [{ name: "uploads" }],
      topics: [{ name: "events" }],
      databases: [{ name: "appdb", engine: "postgres" }]
    },
    extensions: {
      "aws-lambda": {
        region: "us-east-1",
        iam: {
          role: {
            statements: [
              {
                effect: AwsIamEffectEnum.Allow,
                actions: ["s3:PutObject"],
                resources: ["arn:aws:s3:::uploads/*"]
              }
            ]
          }
        }
      }
    }
  };

  await withEnv(
    {
      AWS_ACCESS_KEY_ID: "test",
      AWS_SECRET_ACCESS_KEY: "test",
      AWS_REGION: "us-east-1",
      RUNFABRIC_AWS_REAL_DEPLOY: "1",
      RUNFABRIC_AWS_DEPLOY_CMD: `node ${deployScriptPath}`
    },
    async () => {
      const provider = createAwsLambdaProvider({ projectDir });
      const deployPlan = await provider.planDeploy(project, {
        provider: "aws-lambda",
        entry: "src/index.ts",
        outputPath: "artifact.json"
      });
      const deployResult = await provider.deploy(project, deployPlan);
      assert.equal(deployResult.endpoint, "https://example.lambda-url.us-east-1.on.aws/");
      assert.ok(deployResult.resourceAddresses?.["queue.jobs"]);
      assert.ok(deployResult.workflowAddresses?.["workflow.media-pipeline"]);
      assert.ok(deployResult.secretReferences?.DB_PASSWORD);

      const receiptPath = join(projectDir, ".runfabric", "deploy", "aws-lambda", "deployment.json");
      const receiptContent = await readFile(receiptPath, "utf8");
      const receipt = JSON.parse(receiptContent) as {
        rawResponse?: Record<string, string>;
        resource?: Record<string, unknown>;
      };

      const queueEventSources = JSON.parse(receipt.rawResponse?.queueEventSources || "[]") as Array<Record<string, unknown>>;
      const storageEvents = JSON.parse(receipt.rawResponse?.storageEvents || "[]") as Array<Record<string, unknown>>;
      const eventBridgeRules = JSON.parse(receipt.rawResponse?.eventBridgeRules || "[]") as Array<Record<string, unknown>>;
      const iamRoleStatements = JSON.parse(receipt.rawResponse?.iamRoleStatements || "[]") as Array<Record<string, unknown>>;
      const functionEnv = JSON.parse(receipt.rawResponse?.functionEnv || "{}") as Record<string, string>;
      const resourceAddresses = JSON.parse(receipt.rawResponse?.resourceAddresses || "{}") as Record<string, string>;
      const workflowAddresses = JSON.parse(receipt.rawResponse?.workflowAddresses || "{}") as Record<string, string>;
      const secretReferences = JSON.parse(receipt.rawResponse?.secretReferences || "{}") as Record<string, string>;

      assert.equal(queueEventSources.length, 1);
      assert.equal(queueEventSources[0].queue, "arn:aws:sqs:us-east-1:123456789012:jobs");
      assert.equal(storageEvents.length, 1);
      assert.equal(storageEvents[0].bucket, "uploads");
      assert.equal(eventBridgeRules.length, 1);
      assert.deepEqual(eventBridgeRules[0].pattern, { source: ["runfabric.test"] });
      assert.equal(iamRoleStatements.length, 1);
      assert.equal(iamRoleStatements[0].effect, "Allow");
      assert.equal(functionEnv.BUCKET, "uploads");
      assert.ok(resourceAddresses["queue.jobs"]);
      assert.ok(workflowAddresses["workflow.media-pipeline"]);
      assert.ok(secretReferences.DB_PASSWORD?.includes("arn:aws:secretsmanager"));

      assert.ok(Array.isArray(receipt.resource?.queueEventSources));
      assert.ok(Array.isArray(receipt.resource?.storageEvents));
      assert.ok(Array.isArray(receipt.resource?.eventBridgeRules));
      assert.ok(Array.isArray(receipt.resource?.iamRoleStatements));
      assert.deepEqual(receipt.resource?.functionEnvKeys, ["BUCKET"]);
      assert.ok(isRecord(receipt.resource?.resourceAddresses));
      assert.ok(isRecord(receipt.resource?.workflowAddresses));
      assert.ok(isRecord(receipt.resource?.secretReferences));
    }
  );
});
