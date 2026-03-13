import test from "node:test";
import assert from "node:assert/strict";
import { normalizeRuntimeFamily, normalizeRuntimeMode } from "@runfabric/core";
import { parseProjectConfig } from "@runfabric/planner";

test("normalizeRuntimeFamily supports canonical aliases", () => {
  assert.equal(normalizeRuntimeFamily("nodejs20.x"), "nodejs");
  assert.equal(normalizeRuntimeFamily("python3.12"), "python");
  assert.equal(normalizeRuntimeFamily("golang"), "go");
  assert.equal(normalizeRuntimeFamily("java21"), "java");
  assert.equal(normalizeRuntimeFamily("dot-net8"), "dotnet");
  assert.equal(normalizeRuntimeFamily("c#"), "dotnet");
});

test("normalizeRuntimeMode supports canonical aliases", () => {
  assert.equal(normalizeRuntimeMode("engine"), "engine");
  assert.equal(normalizeRuntimeMode("native"), "native-compat");
  assert.equal(normalizeRuntimeMode("native-compat"), "native-compat");
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
    "    entry: src/app.csproj",
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

test("parseProjectConfig rejects incompatible root runtime and entry", () => {
  const config = [
    "service: runtime-entry-root",
    "runtime: python",
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
    /entry \(src\/index\.ts\) is not compatible with runtime \(python\); expected \.py/
  );
});

test("parseProjectConfig validates function runtime override against effective entry", () => {
  const config = [
    "service: runtime-entry-function",
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
    "    runtime: python",
    ""
  ].join("\n");

  assert.throws(
    () => parseProjectConfig(config),
    /function worker: entry \(src\/index\.ts\) is not compatible with functions\[0\]\.runtime \(python\); expected \.py/
  );
});

test("parseProjectConfig evaluates stage runtime and entry using default-stage baseline", () => {
  const config = [
    "service: runtime-entry-stage-merge",
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
    "stages:",
    "  default:",
    "    runtime: python",
    "    entry: src/default.py",
    "  prod:",
    "    entry: src/prod.py",
    ""
  ].join("\n");

  const parsed = parseProjectConfig(config, { stage: "prod" });
  assert.equal(parsed.runtime, "python");
  assert.equal(parsed.entry, "src/prod.py");
});

test("parseProjectConfig validates stage function entries against stage runtime context", () => {
  const config = [
    "service: runtime-entry-stage-function",
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
    "stages:",
    "  default:",
    "    runtime: python",
    "  prod:",
    "    functions:",
    "      - name: worker",
    "        runtime: java",
    "        entry: src/worker.py",
    ""
  ].join("\n");

  assert.throws(
    () => parseProjectConfig(config, { stage: "prod" }),
    /function worker: stages\.prod\.functions\[0\]\.entry \(src\/worker\.py\) is not compatible with stages\.prod\.functions\[0\]\.runtime \(java\); expected \.java \| \.jar/
  );
});

test("parseProjectConfig parses runtimeMode and stage runtimeMode override", () => {
  const config = [
    "service: runtime-mode",
    "runtime: nodejs",
    "runtimeMode: native",
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
    "stages:",
    "  prod:",
    "    runtimeMode: engine",
    ""
  ].join("\n");

  const parsedDefault = parseProjectConfig(config);
  assert.equal(parsedDefault.runtimeMode, "native-compat");
  const parsedProd = parseProjectConfig(config, { stage: "prod" });
  assert.equal(parsedProd.runtimeMode, "engine");
});

test("parseProjectConfig rejects invalid runtimeMode values", () => {
  const config = [
    "service: runtime-mode-invalid",
    "runtime: nodejs",
    "runtimeMode: hybrid",
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
    /runtimeMode must be one of: native-compat \| engine/
  );
});
