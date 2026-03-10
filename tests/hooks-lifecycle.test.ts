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

test("hooks run before/after build and deploy", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-hooks-"));
  await mkdir(join(projectDir, "src"), { recursive: true });
  await writeFile(
    join(projectDir, "src", "index.ts"),
    "export const handler = async () => ({ status: 200, body: 'ok' });\n",
    "utf8"
  );

  const logPath = join(projectDir, "hook.log");
  await writeFile(
    join(projectDir, "hooks.mjs"),
    [
      "import { appendFileSync } from 'node:fs';",
      `const logPath = ${JSON.stringify(logPath)};`,
      "export default {",
      "  beforeBuild() { appendFileSync(logPath, 'beforeBuild\\n'); },",
      "  afterBuild() { appendFileSync(logPath, 'afterBuild\\n'); },",
      "  beforeDeploy() { appendFileSync(logPath, 'beforeDeploy\\n'); },",
      "  afterDeploy() { appendFileSync(logPath, 'afterDeploy\\n'); }",
      "};",
      ""
    ].join("\n"),
    "utf8"
  );

  await writeFile(
    join(projectDir, "runfabric.yml"),
    [
      "service: hooks-http",
      "runtime: nodejs",
      "entry: src/index.ts",
      "",
      "hooks:",
      "  - ./hooks.mjs",
      "",
      "providers:",
      "  - cloudflare-workers",
      "",
      "triggers:",
      "  - type: http",
      "    method: GET",
      "    path: /hooks",
      ""
    ].join("\n"),
    "utf8"
  );

  const env = {
    CLOUDFLARE_API_TOKEN: "token",
    CLOUDFLARE_ACCOUNT_ID: "account",
    RUNFABRIC_CLOUDFLARE_REAL_DEPLOY: "0"
  };

  const build = runCli(["build", "-c", join(projectDir, "runfabric.yml")], env);
  assert.equal(build.status, 0, build.stderr);

  const deploy = runCli(["deploy", "-c", join(projectDir, "runfabric.yml")], env);
  assert.equal(deploy.status, 0, deploy.stderr);

  const log = await readFile(logPath, "utf8");
  assert.ok(log.includes("beforeBuild"));
  assert.ok(log.includes("afterBuild"));
  assert.ok(log.includes("beforeDeploy"));
  assert.ok(log.includes("afterDeploy"));
});

