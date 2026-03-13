import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, writeFile } from "node:fs/promises";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { spawn, spawnSync } from "node:child_process";
import { once } from "node:events";

const repoRoot = fileURLToPath(new URL("..", import.meta.url));
const cliEntry = join(repoRoot, "apps", "cli", "src", "index.ts");
const runtimeTsConfig = join(repoRoot, "tsconfig.runtime.json");
const tsxBin = join(repoRoot, "node_modules", ".bin", process.platform === "win32" ? "tsx.cmd" : "tsx");
const cliPackageVersion = JSON.parse(readFileSync(join(repoRoot, "apps", "cli", "package.json"), "utf8")).version;

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

test("cli supports --version", () => {
  const result = runCli(["--version"], {});
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout.trim(), cliPackageVersion);
});

test("cli supports version subcommand", () => {
  const result = runCli(["version"], {});
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout.trim(), cliPackageVersion);
});

test("call-local rejects non-node runtime projects", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-call-local-runtime-check-"));
  await mkdir(join(projectDir, "src"), { recursive: true });
  await writeFile(join(projectDir, "src", "index.py"), "def handler(event, context):\n    return {}\n", "utf8");
  await writeFile(
    join(projectDir, "runfabric.yml"),
    [
      "service: runtime-check",
      "runtime: python",
      "entry: src/index.py",
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
  const result = runCli(["call-local", "-c", configPath, "--provider", "aws-lambda", "--method", "GET", "--path", "/hello"], {});
  assert.notEqual(result.status, 0);
  assert.match(
    `${result.stdout}\n${result.stderr}`,
    /call-local currently supports runtime nodejs only\. project runtime is python/
  );
});

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

  const traces = runCli(["traces", "-c", configPath, "--provider", "cloudflare-workers", "--json"], env);
  assert.equal(traces.status, 0, traces.stderr);
  const tracesJson = JSON.parse(traces.stdout);
  assert.ok(Array.isArray(tracesJson.traces));

  const metrics = runCli(["metrics", "-c", configPath, "--provider", "cloudflare-workers", "--json"], env);
  assert.equal(metrics.status, 0, metrics.stderr);
  const metricsJson = JSON.parse(metrics.stdout);
  assert.ok(Array.isArray(metricsJson.metrics));
  assert.ok(metricsJson.metrics.some((metric: { name: string }) => metric.name === "deploy_total"));

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
      env: {
        ...process.env,
        RUNFABRIC_CALL_LOCAL_MAX_BODY_BYTES: "16"
      },
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

    const oversizedResponse = await fetch(`http://127.0.0.1:${port}/hello`, {
      method: "POST",
      body: "abcdefghijklmnopqrstuvwxyz"
    });
    const oversizedBody = await oversizedResponse.text();
    assert.equal(oversizedResponse.status, 413, oversizedBody);
    assert.ok(oversizedBody.includes("request body exceeds 16 bytes"), oversizedBody);
  } finally {
    child.kill("SIGTERM");
    await once(child, "exit");
  }
});

test("call-local --serve supports --event template payload", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-call-local-serve-event-template-"));
  await mkdir(join(projectDir, "src"), { recursive: true });
  await writeFile(
    join(projectDir, "src", "index.ts"),
    [
      "export const handler = async (request) => ({",
      "  status: 200,",
      "  headers: { \"content-type\": \"application/json\" },",
      "  body: JSON.stringify({",
      "    path: request.path,",
      "    principalId: request.raw?.requestContext?.authorizer?.principalId || null",
      "  })",
      "});",
      ""
    ].join("\n"),
    "utf8"
  );
  await writeFile(
    join(projectDir, "event.template.json"),
    JSON.stringify(
      {
        requestContext: {
          authorizer: {
            principalId: "template-user"
          }
        },
        rawPath: "/template-default"
      },
      null,
      2
    ),
    "utf8"
  );
  await writeFile(
    join(projectDir, "runfabric.yml"),
    [
      "service: call-local-serve-template",
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
    [
      "--tsconfig",
      runtimeTsConfig,
      cliEntry,
      "call-local",
      "-c",
      configPath,
      "--provider",
      "aws-lambda",
      "--serve",
      "--event",
      join(projectDir, "event.template.json"),
      "--port",
      "0"
    ],
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
    const response = await fetch(`http://127.0.0.1:${port}/template-check`);
    const payload = (await response.json()) as { path: string; principalId: string | null };

    assert.equal(response.status, 200);
    assert.equal(payload.path, "/template-check");
    assert.equal(payload.principalId, "template-user");
  } finally {
    child.kill("SIGTERM");
    await once(child, "exit");
  }
});

test("call-local prefers built js artifact over ts source entry when available", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-call-local-artifact-priority-"));
  await mkdir(join(projectDir, "src"), { recursive: true });
  await mkdir(join(projectDir, "dist", "src"), { recursive: true });
  await writeFile(
    join(projectDir, "src", "index.ts"),
    [
      "throw new Error('ts source should not be loaded when built js artifact exists');",
      ""
    ].join("\n"),
    "utf8"
  );
  await writeFile(
    join(projectDir, "dist", "src", "index.js"),
    [
      "exports.handler = async () => ({",
      "  status: 200,",
      "  headers: { 'content-type': 'application/json' },",
      "  body: JSON.stringify({ source: 'dist-src-js' })",
      "});",
      ""
    ].join("\n"),
    "utf8"
  );
  await writeFile(
    join(projectDir, "runfabric.yml"),
    [
      "service: artifact-priority",
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
  const result = runCli(
    ["call-local", "-c", configPath, "--provider", "aws-lambda", "--method", "GET", "--path", "/hello"],
    {}
  );

  assert.equal(result.status, 0, result.stderr);
  const output = JSON.parse(result.stdout);
  assert.equal(output.response.statusCode, 200);
  assert.ok(output.response.body.includes("\"source\":\"dist-src-js\""));
});

test("call-local --serve --watch does not start tsc watch when port bind fails", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-call-local-watch-bind-fail-"));
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
    join(projectDir, "tsconfig.json"),
    [
      "{",
      "  \"compilerOptions\": {",
      "    \"target\": \"ES2022\",",
      "    \"module\": \"NodeNext\",",
      "    \"moduleResolution\": \"NodeNext\",",
      "    \"strict\": true,",
      "    \"types\": [\"node\"],",
      "    \"skipLibCheck\": true,",
      "    \"outDir\": \"dist\"",
      "  },",
      "  \"include\": [\"src/**/*.ts\"]",
      "}",
      ""
    ].join("\n"),
    "utf8"
  );
  await writeFile(
    join(projectDir, "runfabric.yml"),
    [
      "service: watch-bind-fail",
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

  const blocker = createServer();
  await new Promise<void>((resolvePromise, rejectPromise) => {
    blocker.once("error", rejectPromise);
    blocker.listen(0, "127.0.0.1", () => resolvePromise());
  });
  const blockedAddress = blocker.address();
  const blockedPort = blockedAddress && typeof blockedAddress === "object" ? blockedAddress.port : 0;

  try {
    const result = runCli(
      [
        "call-local",
        "-c",
        join(projectDir, "runfabric.yml"),
        "--provider",
        "aws-lambda",
        "--serve",
        "--watch",
        "--host",
        "127.0.0.1",
        "--port",
        String(blockedPort)
      ],
      {}
    );
    assert.equal(result.status, 1);
    assert.match(result.stderr, /EADDRINUSE|already in use|listen/i);
    assert.equal(
      result.stdout.includes("watch mode: starting TypeScript compiler"),
      false,
      "tsc watch should not start when server listen fails"
    );
  } finally {
    await new Promise<void>((resolvePromise) => blocker.close(() => resolvePromise()));
  }
});

test("dev preset queue --once runs one local simulation and exits cleanly", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-dev-queue-once-"));
  await mkdir(join(projectDir, "src"), { recursive: true });
  await writeFile(
    join(projectDir, "src", "index.ts"),
    [
      "export const handler = async () => ({",
      "  status: 200,",
      "  headers: { \"content-type\": \"application/json\" },",
      "  body: JSON.stringify({ ok: true, source: \"dev-queue-once\" })",
      "});",
      ""
    ].join("\n"),
    "utf8"
  );
  await writeFile(
    join(projectDir, "runfabric.yml"),
    [
      "service: dev-queue-once",
      "runtime: nodejs",
      "entry: src/index.ts",
      "",
      "providers:",
      "  - aws-lambda",
      "",
      "triggers:",
      "  - type: queue",
      "    queue: jobs",
      ""
    ].join("\n"),
    "utf8"
  );

  const configPath = join(projectDir, "runfabric.yml");
  const result = runCli(["dev", "-c", configPath, "--preset", "queue", "--once", "--no-watch"], {
    AWS_ACCESS_KEY_ID: "test",
    AWS_SECRET_ACCESS_KEY: "test",
    AWS_REGION: "us-east-1"
  });

  assert.equal(result.status, 0, result.stderr);
  assert.ok(result.stdout.includes("queue simulation provider=aws-lambda status=200"));
});
