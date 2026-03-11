import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile } from "node:fs/promises";
import { existsSync } from "node:fs";
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
      "aws-lambda",
      "--lang",
      "ts",
      "--skip-install"
    ]);
    assert.equal(result.status, 0, result.stderr);

    const config = await readFile(join(projectDir, "runfabric.yml"), "utf8");
    assert.ok(
      config.includes(check.expected),
      `template ${check.template} should include "${check.expected}" in runfabric.yml`
    );

    const packageJson = JSON.parse(await readFile(join(projectDir, "package.json"), "utf8"));
    assert.equal(packageJson.dependencies?.["@runfabric/core"], "^0.1.0");
    assert.equal(packageJson.dependencies?.["@runfabric/runtime-node"], undefined);
    assert.equal(
      packageJson.scripts?.["call:local"],
      "runfabric call-local -c runfabric.yml --serve --watch"
    );
    assert.equal(existsSync(join(projectDir, "tsconfig.json")), true);
    assert.equal(existsSync(join(projectDir, "src", "index.ts")), true);
    assert.equal(existsSync(join(projectDir, "scripts", "call-local.mjs")), false);

    const readme = await readFile(join(projectDir, "README.md"), "utf8");
    assert.ok(readme.includes("## Commands"));
    assert.ok(readme.includes("## Local Call (Provider-mimic)"));
    assert.ok(readme.includes("## Credentials"));
    assert.ok(readme.includes("export AWS_ACCESS_KEY_ID"));
  }
});

test("init supports js language scaffold", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-init-js-"));
  const result = runCli([
    "init",
    "--dir",
    projectDir,
    "--template",
    "api",
    "--provider",
    "cloudflare-workers",
    "--lang",
    "js",
    "--skip-install"
  ]);
  assert.equal(result.status, 0, result.stderr);

  const config = await readFile(join(projectDir, "runfabric.yml"), "utf8");
  assert.ok(config.includes("entry: src/index.js"));
  assert.equal(existsSync(join(projectDir, "src", "index.js")), true);
  assert.equal(existsSync(join(projectDir, "tsconfig.json")), false);
  assert.equal(existsSync(join(projectDir, "scripts", "call-local.mjs")), false);

  const packageJson = JSON.parse(await readFile(join(projectDir, "package.json"), "utf8"));
  assert.equal(packageJson.dependencies?.["@runfabric/core"], "^0.1.0");
  assert.equal(packageJson.dependencies?.["@runfabric/runtime-node"], undefined);
  assert.equal(packageJson.scripts?.["call:local"], "runfabric call-local -c runfabric.yml --serve --watch");

  const readme = await readFile(join(projectDir, "README.md"), "utf8");
  assert.ok(readme.includes("src/index.js"));
  assert.ok(readme.includes("call:local"));
  assert.ok(readme.includes("export CLOUDFLARE_API_TOKEN"));
});
