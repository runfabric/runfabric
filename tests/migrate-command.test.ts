import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const repoRoot = fileURLToPath(new URL("..", import.meta.url));
const cliEntry = join(repoRoot, "apps", "cli", "src", "index.ts");
const runtimeTsConfig = join(repoRoot, "tsconfig.runtime.json");
const tsxBin = join(repoRoot, "node_modules", ".bin", process.platform === "win32" ? "tsx.cmd" : "tsx");

function runCli(args: string[]) {
  const result = spawnSync(tsxBin, ["--tsconfig", runtimeTsConfig, cliEntry, ...args], {
    cwd: repoRoot,
    env: process.env,
    encoding: "utf8"
  });
  return {
    status: result.status,
    stdout: result.stdout || "",
    stderr: result.stderr || ""
  };
}

test("migrate converts serverless.yml to runfabric.yml", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-migrate-"));
  await mkdir(projectDir, { recursive: true });
  const inputPath = join(projectDir, "serverless.yml");
  const outputPath = join(projectDir, "runfabric.yml");

  await writeFile(
    inputPath,
    [
      "service: migrate-demo",
      "",
      "provider:",
      "  name: aws",
      "  runtime: nodejs18.x",
      "  region: us-west-1",
      "",
      "functions:",
      "  save:",
      "    handler: src/handler.save",
      "    events:",
      "      - http:",
      "          method: get",
      "          path: /save",
      "      - sqs: arn:aws:sqs:us-east-1:123456789012:jobs",
      "  heartbeat:",
      "    handler: src/cron.run",
      "    events:",
      "      - schedule: cron(0/5 * * * ? *)",
      ""
    ].join("\n"),
    "utf8"
  );

  const migrated = runCli([
    "migrate",
    "--input",
    inputPath,
    "--output",
    outputPath,
    "--json",
    "--force"
  ]);
  assert.equal(migrated.status, 0, migrated.stderr);
  const migratedJson = JSON.parse(migrated.stdout);
  assert.equal(migratedJson.provider, "aws-lambda");
  assert.equal(migratedJson.service, "migrate-demo");
  assert.equal(migratedJson.runtime, "nodejs");

  const runfabricYaml = await readFile(outputPath, "utf8");
  assert.ok(runfabricYaml.includes("service: migrate-demo"));
  assert.ok(runfabricYaml.includes("- aws-lambda"));
  assert.ok(runfabricYaml.includes("type: http"));
  assert.ok(runfabricYaml.includes("type: queue"));
  assert.ok(runfabricYaml.includes("type: cron"));

  const planned = runCli(["plan", "-c", outputPath, "--json"]);
  assert.equal(planned.status, 0, planned.stderr);
});
