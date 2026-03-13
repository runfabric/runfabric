import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { basename, join } from "node:path";
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

function normalizeServiceName(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9-_]/g, "-")
    .replace(/--+/g, "-")
    .replace(/^-+/, "")
    .replace(/-+$/, "") || "runfabric-service";
}

test("init supports all built-in templates", async () => {
  const templateChecks: Array<{
    template: string;
    expected: string;
    provider: string;
    providerPackage: string;
    credentialEnvVar: string;
  }> = [
    {
      template: "api",
      expected: "type: http",
      provider: "aws-lambda",
      providerPackage: "@runfabric/provider-aws-lambda",
      credentialEnvVar: "AWS_ACCESS_KEY_ID"
    },
    {
      template: "worker",
      expected: "path: /work",
      provider: "aws-lambda",
      providerPackage: "@runfabric/provider-aws-lambda",
      credentialEnvVar: "AWS_ACCESS_KEY_ID"
    },
    {
      template: "queue",
      expected: "type: queue",
      provider: "aws-lambda",
      providerPackage: "@runfabric/provider-aws-lambda",
      credentialEnvVar: "AWS_ACCESS_KEY_ID"
    },
    {
      template: "cron",
      expected: "type: cron",
      provider: "aws-lambda",
      providerPackage: "@runfabric/provider-aws-lambda",
      credentialEnvVar: "AWS_ACCESS_KEY_ID"
    },
    {
      template: "storage",
      expected: "type: storage",
      provider: "aws-lambda",
      providerPackage: "@runfabric/provider-aws-lambda",
      credentialEnvVar: "AWS_ACCESS_KEY_ID"
    },
    {
      template: "eventbridge",
      expected: "type: eventbridge",
      provider: "aws-lambda",
      providerPackage: "@runfabric/provider-aws-lambda",
      credentialEnvVar: "AWS_ACCESS_KEY_ID"
    },
    {
      template: "pubsub",
      expected: "type: pubsub",
      provider: "gcp-functions",
      providerPackage: "@runfabric/provider-gcp-functions",
      credentialEnvVar: "GCP_PROJECT_ID"
    }
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
      check.provider,
      "--lang",
      "ts",
      "--skip-install"
    ]);
    assert.equal(result.status, 0, result.stderr);

    const config = await readFile(join(projectDir, "runfabric.yml"), "utf8");
    const expectedService = normalizeServiceName(basename(projectDir));
    assert.ok(
      config.includes(check.expected),
      `template ${check.template} should include "${check.expected}" in runfabric.yml`
    );
    assert.ok(config.includes(`service: ${expectedService}`));
    assert.equal(config.includes(`service: hello-${check.template}`), false);
    assert.ok(config.includes("state:"));
    assert.ok(config.includes("backend: local"));
    assert.ok(config.includes("# yaml-language-server: $schema=./node_modules/@runfabric/core/runfabric.schema.json"));
    const schemaPath = join(projectDir, "node_modules", "@runfabric", "core", "runfabric.schema.json");
    assert.equal(existsSync(schemaPath), true);
    const schema = JSON.parse(await readFile(schemaPath, "utf8"));
    assert.equal(schema.properties?.service?.type, "string");
    assert.equal(Array.isArray(schema.properties?.providers?.items?.enum), true);

    const packageJson = JSON.parse(await readFile(join(projectDir, "package.json"), "utf8"));
    assert.equal(packageJson.dependencies?.["@runfabric/core"], "^0.1.0");
    assert.equal(packageJson.dependencies?.[check.providerPackage], "^0.1.0");
    assert.equal(packageJson.dependencies?.["@runfabric/runtime-node"], undefined);
    assert.equal(
      packageJson.scripts?.["call:local"],
      "runfabric call-local -c runfabric.yml --serve --watch"
    );
    assert.equal(existsSync(join(projectDir, "tsconfig.json")), true);
    assert.equal(existsSync(join(projectDir, "src", "index.ts")), true);
    assert.equal(existsSync(join(projectDir, "scripts", "call-local.mjs")), false);
    assert.equal(existsSync(join(projectDir, ".env.example")), true);

    const envExample = await readFile(join(projectDir, ".env.example"), "utf8");
    assert.ok(envExample.includes(`${check.credentialEnvVar}=your-value`));
    assert.ok(envExample.includes("local backend selected"));

    const readme = await readFile(join(projectDir, "README.md"), "utf8");
    assert.ok(readme.includes("## Commands"));
    assert.ok(readme.includes("## Local Call (Provider-mimic)"));
    assert.ok(readme.includes("## Credentials"));
    assert.ok(readme.includes("cp .env.example .env"));
    assert.ok(readme.includes(`export ${check.credentialEnvVar}`));
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
  assert.ok(config.includes("backend: local"));
  assert.ok(config.includes("# yaml-language-server: $schema=./node_modules/@runfabric/core/runfabric.schema.json"));
  assert.equal(existsSync(join(projectDir, "node_modules", "@runfabric", "core", "runfabric.schema.json")), true);
  assert.equal(existsSync(join(projectDir, "src", "index.js")), true);
  assert.equal(existsSync(join(projectDir, "tsconfig.json")), false);
  assert.equal(existsSync(join(projectDir, "scripts", "call-local.mjs")), false);
  assert.equal(existsSync(join(projectDir, ".env.example")), true);

  const packageJson = JSON.parse(await readFile(join(projectDir, "package.json"), "utf8"));
  assert.equal(packageJson.dependencies?.["@runfabric/core"], "^0.1.0");
  assert.equal(packageJson.dependencies?.["@runfabric/provider-cloudflare-workers"], "^0.1.0");
  assert.equal(packageJson.dependencies?.["@runfabric/runtime-node"], undefined);
  assert.equal(packageJson.scripts?.["call:local"], "runfabric call-local -c runfabric.yml --serve --watch");

  const readme = await readFile(join(projectDir, "README.md"), "utf8");
  assert.ok(readme.includes("src/index.js"));
  assert.ok(readme.includes("call:local"));
  assert.ok(readme.includes("export CLOUDFLARE_API_TOKEN"));

  const envExample = await readFile(join(projectDir, ".env.example"), "utf8");
  assert.ok(envExample.includes("CLOUDFLARE_API_TOKEN=your-value"));
  assert.ok(envExample.includes("CLOUDFLARE_ACCOUNT_ID=your-value"));
});

test("init selects a template-compatible provider when template is specified without provider", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-init-template-provider-auto-"));
  const result = runCli([
    "init",
    "--dir",
    projectDir,
    "--template",
    "pubsub",
    "--lang",
    "ts",
    "--skip-install"
  ]);
  assert.equal(result.status, 0, result.stderr);

  const config = await readFile(join(projectDir, "runfabric.yml"), "utf8");
  assert.ok(config.includes("type: pubsub"));
  assert.ok(config.includes("providers:"));
  assert.ok(config.includes("  - gcp-functions"));

  const packageJson = JSON.parse(await readFile(join(projectDir, "package.json"), "utf8"));
  assert.equal(packageJson.dependencies?.["@runfabric/provider-gcp-functions"], "^0.1.0");
});

test("init supports explicit state backend selection", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-init-state-"));
  const result = runCli([
    "init",
    "--dir",
    projectDir,
    "--template",
    "api",
    "--provider",
    "aws-lambda",
    "--lang",
    "ts",
    "--state-backend",
    "s3",
    "--skip-install"
  ]);
  assert.equal(result.status, 0, result.stderr);

  const config = await readFile(join(projectDir, "runfabric.yml"), "utf8");
  assert.ok(config.includes("backend: s3"));
  assert.ok(config.includes("# yaml-language-server: $schema=./node_modules/@runfabric/core/runfabric.schema.json"));
  assert.ok(config.includes("bucket: ${env:RUNFABRIC_STATE_S3_BUCKET}"));
  assert.ok(config.includes("region: ${env:AWS_REGION,us-east-1}"));
  assert.match(
    config,
    new RegExp(`keyPrefix: runfabric/${normalizeServiceName(basename(projectDir))}-[a-f0-9]{8}/state`)
  );

  const readme = await readFile(join(projectDir, "README.md"), "utf8");
  assert.ok(readme.includes("Configured state backend in `runfabric.yml`: `s3`"));
  assert.ok(readme.includes("RUNFABRIC_STATE_S3_BUCKET"));

  const envExample = await readFile(join(projectDir, ".env.example"), "utf8");
  assert.ok(envExample.includes("RUNFABRIC_STATE_S3_BUCKET=your-state-bucket"));
  assert.ok(envExample.includes("AWS_REGION=us-east-1"));
});

test("init namespaces object-storage state backends with random hash", async () => {
  const backends = ["s3", "gcs", "azblob"] as const;

  for (const backend of backends) {
    const projectDir = await mkdtemp(join(tmpdir(), `runfabric-init-${backend}-`));
    const result = runCli([
      "init",
      "--dir",
      projectDir,
      "--template",
      "api",
      "--provider",
      "aws-lambda",
      "--lang",
      "ts",
      "--state-backend",
      backend,
      "--skip-install"
    ]);
    assert.equal(result.status, 0, result.stderr);

    const config = await readFile(join(projectDir, "runfabric.yml"), "utf8");
    const fieldName = backend === "s3" ? "keyPrefix" : "prefix";
    assert.match(
      config,
      new RegExp(`${fieldName}: runfabric/${normalizeServiceName(basename(projectDir))}-[a-f0-9]{8}/state`)
    );
  }
});

test("init rejects template not supported by selected provider", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-init-template-provider-mismatch-"));
  const result = runCli([
    "init",
    "--dir",
    projectDir,
    "--template",
    "queue",
    "--provider",
    "cloudflare-workers",
    "--lang",
    "ts",
    "--skip-install"
  ]);
  assert.notEqual(result.status, 0);
  const combinedOutput = `${result.stdout}\n${result.stderr}`;
  assert.match(
    combinedOutput,
    /template "queue" is not supported by provider "cloudflare-workers"/
  );
});

test("init rejects template not supported by any provider", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-init-template-unsupported-"));
  const result = runCli([
    "init",
    "--dir",
    projectDir,
    "--template",
    "kafka",
    "--lang",
    "ts",
    "--skip-install",
    "--no-interactive"
  ]);
  assert.notEqual(result.status, 0);
  const combinedOutput = `${result.stdout}\n${result.stderr}`;
  assert.match(combinedOutput, /no providers support template "kafka"/);
});
