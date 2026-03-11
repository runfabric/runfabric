import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { spawn, spawnSync } from "node:child_process";
import { once } from "node:events";

const repoRoot = fileURLToPath(new URL("..", import.meta.url));
const cliEntry = join(repoRoot, "apps", "cli", "src", "index.ts");
const runtimeTsConfig = join(repoRoot, "tsconfig.runtime.json");
const tsxBin = join(repoRoot, "node_modules", ".bin", process.platform === "win32" ? "tsx.cmd" : "tsx");

function runCli(args: string[], env: Record<string, string>): { status: number | null; stdout: string; stderr: string } {
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

test("cli smoke: doctor/plan/build/deploy complete for cloudflare fixture", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-cli-smoke-"));
  await mkdir(join(projectDir, "src"), { recursive: true });
  await writeFile(
    join(projectDir, "src", "index.ts"),
    [
      "export const handler = async () => ({",
      "  status: 200,",
      "  headers: { \"content-type\": \"application/json\" },",
      "  body: JSON.stringify({ ok: true })",
      "});",
      ""
    ].join("\n"),
    "utf8"
  );
  await writeFile(
    join(projectDir, "runfabric.yml"),
    [
      "service: smoke-http",
      "runtime: nodejs",
      "entry: src/index.ts",
      "",
      "providers:",
      "  - cloudflare-workers",
      "",
      "triggers:",
      "  - type: http",
      "    method: GET",
      "    path: /smoke",
      ""
    ].join("\n"),
    "utf8"
  );

  const configPath = join(projectDir, "runfabric.yml");
  const env = {
    CLOUDFLARE_API_TOKEN: "test-token",
    CLOUDFLARE_ACCOUNT_ID: "test-account",
    RUNFABRIC_CLOUDFLARE_REAL_DEPLOY: "0"
  };

  const doctor = runCli(["doctor", "-c", configPath], env);
  assert.equal(doctor.status, 0, doctor.stderr);

  const plan = runCli(["plan", "-c", configPath, "--json"], env);
  assert.equal(plan.status, 0, plan.stderr);
  const planJson = JSON.parse(plan.stdout);
  assert.equal(planJson.ok, true);
  assert.equal(planJson.providerPlans.length, 1);

  const buildOut = join(projectDir, ".tmp-build");
  const build = runCli(["build", "-c", configPath, "-o", buildOut, "--json"], env);
  assert.equal(build.status, 0, build.stderr);
  const buildJson = JSON.parse(build.stdout);
  assert.equal(buildJson.artifacts.length, 1);
  assert.ok(existsSync(buildJson.artifacts[0].outputPath));
  assert.ok(existsSync(buildJson.artifacts[0].entry));

  const localCall = runCli(
    ["call-local", "-c", configPath, "--provider", "cloudflare-workers", "--method", "GET", "--path", "/smoke"],
    env
  );
  assert.equal(localCall.status, 0, localCall.stderr);
  const localCallJson = JSON.parse(localCall.stdout);
  assert.equal(localCallJson.provider, "cloudflare-workers");
  assert.equal(localCallJson.response.statusCode, 200);
  assert.ok(typeof localCallJson.response.body === "string");

  const deploy = runCli(["deploy", "-c", configPath, "-o", buildOut, "--json"], env);
  assert.equal(deploy.status, 0, deploy.stderr);
  const deployJson = JSON.parse(deploy.stdout);
  assert.equal(deployJson.deployments.length, 1);
  assert.ok(deployJson.deployments[0].endpoint.includes(".workers.dev"));

  const receiptPath = join(projectDir, ".runfabric", "deploy", "cloudflare-workers", "deployment.json");
  const receipt = JSON.parse(await readFile(receiptPath, "utf8"));
  assert.equal(receipt.mode, "simulated");

  const statePath = join(
    projectDir,
    ".runfabric",
    "state",
    "smoke-http",
    "default",
    "cloudflare-workers.state.json"
  );
  const state = JSON.parse(await readFile(statePath, "utf8"));
  assert.equal(state.provider, "cloudflare-workers");
});

test("call-local --serve starts localhost server", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-call-local-serve-"));
  await mkdir(join(projectDir, "src"), { recursive: true });
  await writeFile(
    join(projectDir, "src", "index.ts"),
    [
      "export const handler = async () => ({",
      "  status: 200,",
      "  headers: { \"content-type\": \"application/json\" },",
      "  body: JSON.stringify({ ok: true, source: \"serve\" })",
      "});",
      ""
    ].join("\n"),
    "utf8"
  );
  await writeFile(
    join(projectDir, "runfabric.yml"),
    [
      "service: call-local-serve",
      "runtime: nodejs",
      "entry: src/index.ts",
      "",
      "providers:",
      "  - aws-lambda",
      "",
      "triggers:",
      "  - type: http",
      "    method: GET",
      "    path: /hello",
      ""
    ].join("\n"),
    "utf8"
  );

  const configPath = join(projectDir, "runfabric.yml");
  const child = spawn(
    tsxBin,
    ["--tsconfig", runtimeTsConfig, cliEntry, "call-local", "-c", configPath, "--provider", "aws-lambda", "--serve", "--port", "0"],
    {
      cwd: repoRoot,
      env: process.env,
      stdio: ["ignore", "pipe", "pipe"]
    }
  );

  let combinedOutput = "";
  child.stdout.setEncoding("utf8");
  child.stderr.setEncoding("utf8");
  child.stdout.on("data", (chunk: string) => {
    combinedOutput += chunk;
  });
  child.stderr.on("data", (chunk: string) => {
    combinedOutput += chunk;
  });

  const port = await new Promise<number>((resolvePromise, rejectPromise) => {
    const timeout = setTimeout(() => {
      cleanup();
      rejectPromise(new Error(`timed out waiting for call-local server\n${combinedOutput}`));
    }, 10_000);

    const onData = (): void => {
      const match = combinedOutput.match(/local call server listening on http:\/\/127\.0\.0\.1:(\d+)/);
      if (!match) {
        return;
      }
      cleanup();
      resolvePromise(Number(match[1]));
    };

    const onExit = (code: number | null): void => {
      cleanup();
      rejectPromise(new Error(`call-local server exited early (${code ?? "unknown"})\n${combinedOutput}`));
    };

    const cleanup = (): void => {
      clearTimeout(timeout);
      child.stdout.off("data", onData);
      child.off("exit", onExit);
    };

    child.stdout.on("data", onData);
    child.on("exit", onExit);
    onData();
  });

  try {
    const response = await fetch(`http://127.0.0.1:${port}/hello`);
    const body = await response.text();
    assert.equal(response.status, 200, body);
    assert.ok(body.includes("\"source\":\"serve\""), body);
  } finally {
    child.kill("SIGTERM");
    await once(child, "exit");
  }
});
