import test from "node:test";
import assert from "node:assert/strict";
import { TriggerEnum } from "../packages/core/src/index.ts";
import type { ProjectConfig } from "../packages/core/src/index.ts";
import { createPlan } from "../packages/planner/src/planner.ts";

function makeProject(providers: string[], triggerType: "queue" | "cron"): ProjectConfig {
  return {
    service: `trigger-${triggerType}`,
    runtime: "nodejs",
    entry: "src/index.ts",
    providers,
    triggers:
      triggerType === "queue"
        ? [{ type: TriggerEnum.Queue, queue: "jobs" }]
        : [{ type: TriggerEnum.Cron, schedule: "rate(5 minutes)" }]
  };
}

test("queue trigger support differs by provider capabilities", () => {
  const supported = createPlan(makeProject(["aws-lambda"], "queue"));
  assert.equal(supported.ok, true);

  const unsupported = createPlan(makeProject(["cloudflare-workers"], "queue"));
  assert.equal(unsupported.ok, false);
  assert.ok(unsupported.errors.some((message) => message.includes("queue trigger is not supported")));
});

test("cron trigger support differs by provider capabilities", () => {
  const supported = createPlan(makeProject(["gcp-functions"], "cron"));
  assert.equal(supported.ok, true);

  const unsupported = createPlan(makeProject(["fly-machines"], "cron"));
  assert.equal(unsupported.ok, false);
  assert.ok(unsupported.errors.some((message) => message.includes("cron trigger is not supported")));
});

test("eventbridge and pubsub trigger support differs by provider capabilities", () => {
  const eventBridgeSupported = createPlan({
    service: "eventbridge-supported",
    runtime: "nodejs",
    entry: "src/index.ts",
    providers: ["aws-lambda"],
    triggers: [{ type: TriggerEnum.EventBridge, pattern: { source: ["runfabric"] } }]
  });
  assert.equal(eventBridgeSupported.ok, true);

  const eventBridgeUnsupported = createPlan({
    service: "eventbridge-unsupported",
    runtime: "nodejs",
    entry: "src/index.ts",
    providers: ["gcp-functions"],
    triggers: [{ type: TriggerEnum.EventBridge, pattern: { source: ["runfabric"] } }]
  });
  assert.equal(eventBridgeUnsupported.ok, false);
  assert.ok(
    eventBridgeUnsupported.errors.some((message) => message.includes("eventbridge trigger is not supported"))
  );

  const pubsubSupported = createPlan({
    service: "pubsub-supported",
    runtime: "nodejs",
    entry: "src/index.ts",
    providers: ["gcp-functions"],
    triggers: [{ type: TriggerEnum.PubSub, topic: "projects/test/topics/events" }]
  });
  assert.equal(pubsubSupported.ok, true);

  const pubsubUnsupported = createPlan({
    service: "pubsub-unsupported",
    runtime: "nodejs",
    entry: "src/index.ts",
    providers: ["aws-lambda"],
    triggers: [{ type: TriggerEnum.PubSub, topic: "projects/test/topics/events" }]
  });
  assert.equal(pubsubUnsupported.ok, false);
  assert.ok(pubsubUnsupported.errors.some((message) => message.includes("pubsub trigger is not supported")));
});
