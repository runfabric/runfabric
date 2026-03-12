import test from "node:test";
import assert from "node:assert/strict";
import type { ProjectConfig } from "../packages/core/src/index.ts";
import { TriggerEnum } from "../packages/core/src/index.ts";
import { resolveFunctionProject } from "../apps/cli/src/utils/project-functions.ts";

test("resolveFunctionProject merges root env with function env override", () => {
  const project: ProjectConfig = {
    service: "env-merge",
    runtime: "nodejs",
    entry: "src/index.ts",
    providers: ["aws-lambda"],
    triggers: [{ type: TriggerEnum.Http, method: "GET", path: "/" }],
    env: {
      SHARED: "root",
      ROOT_ONLY: "1"
    },
    functions: [
      {
        name: "worker",
        entry: "src/worker.ts",
        triggers: [{ type: TriggerEnum.Queue, queue: "jobs" }],
        env: {
          SHARED: "fn",
          FN_ONLY: "1"
        }
      }
    ]
  };

  const resolved = resolveFunctionProject(project, "worker");
  assert.equal(resolved.entry, "src/worker.ts");
  assert.equal(resolved.triggers[0].type, "queue");
  assert.equal(resolved.env?.SHARED, "fn");
  assert.equal(resolved.env?.ROOT_ONLY, "1");
  assert.equal(resolved.env?.FN_ONLY, "1");
});
