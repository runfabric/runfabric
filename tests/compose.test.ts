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

async function writeServiceConfig(rootDir: string, serviceName: string): Promise<string> {
  const serviceDir = join(rootDir, serviceName);
  await mkdir(join(serviceDir, "src"), { recursive: true });
  await writeFile(
    join(serviceDir, "src", "index.ts"),
    "export const handler = async () => ({ status: 200, body: 'ok' });\n",
    "utf8"
  );
  const configPath = join(serviceDir, "runfabric.yml");
  await writeFile(
    configPath,
    [
      `service: ${serviceName}`,
      "runtime: nodejs",
      "entry: src/index.ts",
      "",
      "providers:",
      "  - cloudflare-workers",
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

test("compose deploy executes services in dependency order and exports outputs", async () => {
  const workspace = await mkdtemp(join(tmpdir(), "runfabric-compose-"));
  const apiConfig = await writeServiceConfig(workspace, "svc-api");
  const workerConfig = await writeServiceConfig(workspace, "svc-worker");

  const composePath = join(workspace, "runfabric.compose.yml");
  await writeFile(
    composePath,
    [
      "services:",
      "  - name: api",
      `    config: ${apiConfig}`,
      "  - name: worker",
      `    config: ${workerConfig}`,
      "    dependsOn:",
      "      - api",
      ""
    ].join("\n"),
    "utf8"
  );

  const result = runCli(
    ["compose", "deploy", "-f", composePath, "--json"],
    {
      CLOUDFLARE_API_TOKEN: "token",
      CLOUDFLARE_ACCOUNT_ID: "account",
      RUNFABRIC_CLOUDFLARE_REAL_DEPLOY: "0"
    }
  );

  assert.equal(result.status, 0, result.stderr);
  const payload = JSON.parse(result.stdout);
  assert.deepEqual(payload.order, ["api", "worker"]);
  assert.equal(payload.services.length, 2);
  assert.ok(Object.keys(payload.sharedOutputs).some((key) => key.includes("RUNFABRIC_OUTPUT_API_CLOUDFLARE_WORKERS_ENDPOINT")));
});

