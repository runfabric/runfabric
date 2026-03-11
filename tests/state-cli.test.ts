import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { createStateBackend } from "../packages/core/src/state.ts";

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

async function createProject(projectDir: string, stateLines: string[]): Promise<string> {
  await mkdir(join(projectDir, "src"), { recursive: true });
  await writeFile(
    join(projectDir, "src", "index.ts"),
    "export const handler = async () => ({ status: 200, body: 'ok' });\n",
    "utf8"
  );

  const configPath = join(projectDir, "runfabric.yml");
  await writeFile(
    configPath,
    [
      "service: state-cli",
      "runtime: nodejs",
      "entry: src/index.ts",
      "",
      "providers:",
      "  - cloudflare-workers",
      "",
      ...stateLines,
      "",
      "triggers:",
      "  - type: http",
      "    method: GET",
      "    path: /hello",
      ""
    ].join("\n"),
    "utf8"
  );
  return configPath;
}

const env = {
  CLOUDFLARE_API_TOKEN: "token",
  CLOUDFLARE_ACCOUNT_ID: "account",
  RUNFABRIC_CLOUDFLARE_REAL_DEPLOY: "0"
};

const runRemoteStateIntegration = process.env.RUNFABRIC_TEST_REMOTE_STATE === "1";

test(
  "deploy/remove works with postgres state backend",
  { skip: !runRemoteStateIntegration },
  async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-state-postgres-"));
  const configPath = await createProject(projectDir, [
    "state:",
    "  backend: postgres",
    "  postgres:",
    "    schema: public",
    "    table: runfabric_state"
  ]);

  const deploy = runCli(["deploy", "-c", configPath, "--json"], env);
  assert.equal(deploy.status, 0, deploy.stderr);

  const listBeforeRemove = runCli(["state", "list", "-c", configPath, "--json"], env);
  assert.equal(listBeforeRemove.status, 0, listBeforeRemove.stderr);
  const listBeforeRemoveJson = JSON.parse(listBeforeRemove.stdout);
  assert.equal(listBeforeRemoveJson.backend, "postgres");
  assert.equal(listBeforeRemoveJson.records.length, 1);

  const remove = runCli(["remove", "-c", configPath, "--json"], env);
  assert.equal(remove.status, 0, remove.stderr);

  const listAfterRemove = runCli(["state", "list", "-c", configPath, "--json"], env);
  assert.equal(listAfterRemove.status, 0, listAfterRemove.stderr);
  const listAfterRemoveJson = JSON.parse(listAfterRemove.stdout);
  assert.equal(listAfterRemoveJson.records.length, 0);
  }
);

test("deploy/remove works with s3 state backend", { skip: !runRemoteStateIntegration }, async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-state-s3-"));
  const configPath = await createProject(projectDir, [
    "state:",
    "  backend: s3",
    "  s3:",
    "    bucket: state-bucket",
    "    region: us-east-1",
    "    keyPrefix: runfabric/state"
  ]);

  const deploy = runCli(["deploy", "-c", configPath, "--json"], env);
  assert.equal(deploy.status, 0, deploy.stderr);

  const listBeforeRemove = runCli(["state", "list", "-c", configPath, "--json"], env);
  assert.equal(listBeforeRemove.status, 0, listBeforeRemove.stderr);
  const listBeforeRemoveJson = JSON.parse(listBeforeRemove.stdout);
  assert.equal(listBeforeRemoveJson.backend, "s3");
  assert.equal(listBeforeRemoveJson.records.length, 1);

  const remove = runCli(["remove", "-c", configPath, "--json"], env);
  assert.equal(remove.status, 0, remove.stderr);

  const listAfterRemove = runCli(["state", "list", "-c", configPath, "--json"], env);
  assert.equal(listAfterRemove.status, 0, listAfterRemove.stderr);
  const listAfterRemoveJson = JSON.parse(listAfterRemove.stdout);
  assert.equal(listAfterRemoveJson.records.length, 0);
});

test("state force-unlock flow", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-state-ops-"));
  const configPath = await createProject(projectDir, [
    "state:",
    "  backend: local",
    "  lock:",
    "    timeoutSeconds: 30"
  ]);

  const deploy = runCli(["deploy", "-c", configPath, "--json"], env);
  assert.equal(deploy.status, 0, deploy.stderr);

  const backend = createStateBackend({
    projectDir,
    state: { backend: "local" }
  });
  const address = {
    service: "state-cli",
    stage: "default",
    provider: "cloudflare-workers"
  };
  await backend.lock(address, "test-lock-owner");

  const forceUnlock = runCli(
    [
      "state",
      "force-unlock",
      "-c",
      configPath,
      "--service",
      "state-cli",
      "--provider",
      "cloudflare-workers",
      "--json"
    ],
    env
  );
  assert.equal(forceUnlock.status, 0, forceUnlock.stderr);
  const forceUnlockJson = JSON.parse(forceUnlock.stdout);
  assert.equal(forceUnlockJson.removed, true);
});

test("state migrate and reconcile flows on local backends", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-state-migrate-local-"));
  const configPath = await createProject(projectDir, [
    "state:",
    "  backend: local",
    "  local:",
    "    dir: ./.runfabric/state-source",
    "  lock:",
    "    timeoutSeconds: 30"
  ]);

  const deploy = runCli(["deploy", "-c", configPath, "--json"], env);
  assert.equal(deploy.status, 0, deploy.stderr);

  const migrate = runCli(
    ["state", "migrate", "-c", configPath, "--from", "local", "--to", "local", "--json"],
    env
  );
  assert.notEqual(migrate.status, 0);

  const reconcile = runCli(
    ["state", "reconcile", "-c", configPath, "--backend", "local", "--json"],
    env
  );
  assert.equal(reconcile.status, 0, reconcile.stderr);
  const reconcileJson = JSON.parse(reconcile.stdout);
  assert.equal(reconcileJson.summary.driftCount, 0);
});
