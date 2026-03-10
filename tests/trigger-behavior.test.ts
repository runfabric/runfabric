import test from "node:test";
import assert from "node:assert/strict";
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
        ? [{ type: "queue", queue: "jobs" }]
        : [{ type: "cron", schedule: "rate(5 minutes)" }]
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

