import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const repoRoot = fileURLToPath(new URL("..", import.meta.url));
const cliEntry = join(repoRoot, "apps", "cli", "src", "index.ts");
const runtimeTsConfig = join(repoRoot, "tsconfig.runtime.json");
const tsxBin = join(repoRoot, "node_modules", ".bin", process.platform === "win32" ? "tsx.cmd" : "tsx");

function runCli(args: string[], env: Record<string, string>) {
  const result = spawnSync(tsxBin, ["--tsconfig", runtimeTsConfig, cliEntry, ...args], {
    cwd: repoRoot,
    env: {
      ...process.env,
      ...env
    },
    encoding: "utf8"
  });
  return {
    status: result.status,
    stdout: result.stdout || "",
    stderr: result.stderr || ""
  };
}

test("package + deploy function + remove workflow", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-lifecycle-"));
  await mkdir(join(projectDir, "src"), { recursive: true });
  await writeFile(
    join(projectDir, "src", "index.ts"),
    "export const handler = async () => ({ status: 200, body: 'root' });\n",
    "utf8"
  );
  await writeFile(
    join(projectDir, "src", "api.ts"),
    "export const handler = async () => ({ status: 200, body: 'api' });\n",
    "utf8"
  );
  await writeFile(
    join(projectDir, "runfabric.yml"),
    [
      "service: lifecycle-http",
      "runtime: nodejs",
      "entry: src/index.ts",
      "",
      "providers:",
      "  - cloudflare-workers",
      "",
      "functions:",
      "  - name: api",
      "    entry: src/api.ts",
      "    runtime: nodejs",
      "    triggers:",
      "      - type: http",
      "        method: GET",
      "        path: /api",
      "",
      "triggers:",
      "  - type: http",
      "    method: GET",
      "    path: /",
      ""
    ].join("\n"),
    "utf8"
  );

  const env = {
    CLOUDFLARE_API_TOKEN: "token",
    CLOUDFLARE_ACCOUNT_ID: "account",
    RUNFABRIC_CLOUDFLARE_REAL_DEPLOY: "0"
  };

  const packaged = runCli(
    ["package", "-c", join(projectDir, "runfabric.yml"), "-f", "api", "--json"],
    env
  );
  assert.equal(packaged.status, 0, packaged.stderr);
  const packageJson = JSON.parse(packaged.stdout);
  assert.equal(packageJson.function, "api");
  assert.ok(packageJson.artifacts.length >= 1);

  const deployed = runCli(
    ["deploy-function", "api", "-c", join(projectDir, "runfabric.yml"), "--json"],
    env
  );
  assert.equal(deployed.status, 0, deployed.stderr);
  const deployJson = JSON.parse(deployed.stdout);
  assert.equal(deployJson.summary.exitCode, 0);

  const receiptPath = join(projectDir, ".runfabric", "deploy", "cloudflare-workers", "deployment.json");
  assert.ok(existsSync(receiptPath));
  const receipt = JSON.parse(await readFile(receiptPath, "utf8"));
  assert.equal(receipt.provider, "cloudflare-workers");

  const removed = runCli(["remove", "-c", join(projectDir, "runfabric.yml"), "--json"], env);
  assert.equal(removed.status, 0, removed.stderr);
  const removeJson = JSON.parse(removed.stdout);
  assert.equal(removeJson.failures.length, 0);
  assert.equal(existsSync(receiptPath), false);
});
