import test from "node:test";
import assert from "node:assert/strict";
import { parseProjectConfig } from "../packages/planner/src/parse-config.ts";
import { createPrimitiveCompatibilityReport } from "../packages/planner/src/primitive-compatibility.ts";
import { primitiveProfiles } from "../packages/planner/src/primitive-profiles.ts";
import { createPlan } from "../packages/planner/src/planner.ts";

test("primitive compatibility report marks partial support", () => {
  const report = createPrimitiveCompatibilityReport(
    ["aws-lambda", "cloudflare-workers"],
    {
      "aws-lambda": primitiveProfiles["aws-lambda"],
      "cloudflare-workers": primitiveProfiles["cloudflare-workers"]
    }
  );

  assert.ok(report.universallySupported.includes("compute"));
  assert.ok(report.partiallySupported.includes("queue"));
  assert.ok(report.providerGaps["cloudflare-workers"].includes("queue"));
});

test("createPlan includes primitive compatibility report", () => {
  const config = [
    "service: primitive-check",
    "runtime: nodejs",
    "entry: src/index.ts",
    "",
    "providers:",
    "  - aws-lambda",
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

  assert.ok(plan.primitiveCompatibility.universallySupported.includes("compute"));
  assert.ok(plan.primitiveCompatibility.partiallySupported.includes("queue"));
});
