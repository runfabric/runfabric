import test from "node:test";
import assert from "node:assert/strict";
import { normalizeRuntimeFamily } from "@runfabric/core";
import { parseProjectConfig } from "@runfabric/planner";

test("normalizeRuntimeFamily supports canonical aliases", () => {
  assert.equal(normalizeRuntimeFamily("nodejs20.x"), "nodejs");
  assert.equal(normalizeRuntimeFamily("python3.12"), "python");
  assert.equal(normalizeRuntimeFamily("golang"), "go");
  assert.equal(normalizeRuntimeFamily("java21"), "java");
  assert.equal(normalizeRuntimeFamily("dot-net8"), "dotnet");
  assert.equal(normalizeRuntimeFamily("c#"), "dotnet");
});

test("parseProjectConfig normalizes runtime families for project/function/stage", () => {
  const config = [
    "service: runtime-normalization",
    "runtime: nodejs20.x",
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
    "functions:",
    "  - name: worker",
    "    runtime: python3.12",
    "    entry: src/worker.py",
    "",
    "stages:",
    "  prod:",
    "    runtime: dotnet8",
    ""
  ].join("\n");

  const parsedDefault = parseProjectConfig(config);
  assert.equal(parsedDefault.runtime, "nodejs");
  assert.equal(parsedDefault.functions?.[0]?.runtime, "python");

  const parsedProd = parseProjectConfig(config, { stage: "prod" });
  assert.equal(parsedProd.runtime, "dotnet");
  assert.equal(parsedProd.functions?.[0]?.runtime, "python");
});

test("parseProjectConfig reports path-specific runtime validation errors", () => {
  const config = [
    "service: runtime-errors",
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
    "functions:",
    "  - name: worker",
    "    runtime: ruby",
    "    entry: src/worker.rb",
    "",
    "stages:",
    "  prod:",
    "    runtime: ruby",
    ""
  ].join("\n");

  assert.throws(
    () => parseProjectConfig(config),
    /functions\[0\]\.runtime must be one of: nodejs \| python \| go \| java \| rust \| dotnet/
  );
  assert.throws(
    () => parseProjectConfig(config, { stage: "prod" }),
    /stages\.prod\.runtime must be one of: nodejs \| python \| go \| java \| rust \| dotnet/
  );
});
