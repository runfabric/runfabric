import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const repoRoot = fileURLToPath(new URL("..", import.meta.url));
const cliEntry = join(repoRoot, "apps", "cli", "src", "index.ts");
const runtimeTsConfig = join(repoRoot, "tsconfig.runtime.json");
const tsxBin = join(repoRoot, "node_modules", ".bin", process.platform === "win32" ? "tsx.cmd" : "tsx");

function runCli(args: string[]): { status: number | null; stdout: string; stderr: string } {
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

test("init supports api/worker/queue/cron templates", async () => {
  const templateChecks: Array<{ template: string; expected: string }> = [
    { template: "api", expected: "type: http" },
    { template: "worker", expected: "path: /work" },
    { template: "queue", expected: "type: queue" },
    { template: "cron", expected: "type: cron" }
  ];

  for (const check of templateChecks) {
    const projectDir = await mkdtemp(join(tmpdir(), `runfabric-init-${check.template}-`));
    const result = runCli([
      "init",
      "--dir",
      projectDir,
      "--template",
      check.template,
      "--provider",
      "aws-lambda"
    ]);
    assert.equal(result.status, 0, result.stderr);

    const config = await readFile(join(projectDir, "runfabric.yml"), "utf8");
    assert.ok(
      config.includes(check.expected),
      `template ${check.template} should include "${check.expected}" in runfabric.yml`
    );
  }
});
