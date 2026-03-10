import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const repoRoot = fileURLToPath(new URL("..", import.meta.url));
const cliEntry = join(repoRoot, "apps", "cli", "src", "index.ts");
const runtimeTsConfig = join(repoRoot, "tsconfig.runtime.json");
const tsxBin = join(repoRoot, "node_modules", ".bin", process.platform === "win32" ? "tsx.cmd" : "tsx");

function runDeploy(
  configPath: string,
  env: Record<string, string>
): { status: number | null; stdout: string; stderr: string } {
  const result = spawnSync(
    tsxBin,
    ["--tsconfig", runtimeTsConfig, cliEntry, "deploy", "-c", configPath, "--json"],
    {
      cwd: repoRoot,
      env: {
        ...process.env,
        ...env
      },
      encoding: "utf8"
    }
  );

  return {
    status: result.status,
    stdout: result.stdout || "",
    stderr: result.stderr || ""
  };
}

test("deploy returns exit code 2 on partial provider failures", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-partial-deploy-"));
  await mkdir(join(projectDir, "src"), { recursive: true });
  await writeFile(join(projectDir, "src", "index.ts"), "export const handler = async () => ({ ok: true });\n");
  await writeFile(
    join(projectDir, "runfabric.yml"),
    [
      "service: partial-deploy",
      "runtime: nodejs",
      "entry: src/index.ts",
      "",
      "providers:",
      "  - cloudflare-workers",
      "  - vercel",
      "",
      "triggers:",
      "  - type: http",
      "    method: GET",
      "    path: /partial",
      ""
    ].join("\n"),
    "utf8"
  );

  const result = runDeploy(join(projectDir, "runfabric.yml"), {
    CLOUDFLARE_API_TOKEN: "token",
    CLOUDFLARE_ACCOUNT_ID: "account",
    RUNFABRIC_CLOUDFLARE_REAL_DEPLOY: "0"
  });

  assert.equal(result.status, 2, result.stderr);
  const json = JSON.parse(result.stdout);
  assert.equal(json.summary.exitCode, 2);
  assert.equal(json.deployments.length, 1);
  assert.ok(json.failures.length >= 1);
});

test("deploy returns exit code 1 when all providers fail", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-full-fail-deploy-"));
  await mkdir(join(projectDir, "src"), { recursive: true });
  await writeFile(join(projectDir, "src", "index.ts"), "export const handler = async () => ({ ok: true });\n");
  await writeFile(
    join(projectDir, "runfabric.yml"),
    [
      "service: full-fail-deploy",
      "runtime: nodejs",
      "entry: src/index.ts",
      "",
      "providers:",
      "  - vercel",
      "",
      "triggers:",
      "  - type: http",
      "    method: GET",
      "    path: /full-fail",
      ""
    ].join("\n"),
    "utf8"
  );

  const result = runDeploy(join(projectDir, "runfabric.yml"), {});
  assert.equal(result.status, 1, result.stderr);
  const json = JSON.parse(result.stdout);
  assert.equal(json.summary.exitCode, 1);
  assert.equal(json.deployments.length, 0);
  assert.ok(json.failures.length >= 1);
});
