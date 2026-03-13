import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { ProjectConfig, ProviderAdapter, RuntimeFamily } from "../packages/core/src/index.ts";
import { TriggerEnum } from "../packages/core/src/index.ts";
import { createAwsLambdaProvider } from "../packages/provider-aws-lambda/src/index.ts";
import { createGcpFunctionsProvider } from "../packages/provider-gcp-functions/src/index.ts";
import { createIbmOpenWhiskProvider } from "../packages/provider-ibm-openwhisk/src/index.ts";

function withEnv(overrides: Record<string, string>, fn: () => Promise<void>): Promise<void> {
  const previous = new Map<string, string | undefined>();
  for (const [key, value] of Object.entries(overrides)) {
    previous.set(key, process.env[key]);
    process.env[key] = value;
  }
  return fn().finally(() => {
    for (const [key, value] of previous.entries()) {
      if (value === undefined) {
        delete process.env[key];
      } else {
        process.env[key] = value;
      }
    }
  });
}

function createProject(provider: string, runtime: RuntimeFamily): ProjectConfig {
  return {
    service: `runtime-map-${provider.replace(/[^a-z0-9-]/gi, "-")}-${runtime}`,
    runtime,
    entry: "src/index.ts",
    stage: "ci",
    providers: [provider],
    triggers: [{ type: TriggerEnum.Http, method: "GET", path: "/runtime-map" }]
  };
}

function nodeScriptCommand(scriptPath: string): string {
  return `node ${JSON.stringify(scriptPath)}`;
}

async function writeCaptureScript(
  scriptPath: string,
  capturePath: string,
  response: Record<string, unknown>
): Promise<void> {
  const script = [
    "const fs = require('node:fs');",
    "const payload = {",
    "  RUNFABRIC_AWS_RUNTIME: process.env.RUNFABRIC_AWS_RUNTIME || '',",
    "  RUNFABRIC_GCP_RUNTIME: process.env.RUNFABRIC_GCP_RUNTIME || '',",
    "  RUNFABRIC_IBM_RUNTIME_KIND: process.env.RUNFABRIC_IBM_RUNTIME_KIND || ''",
    "};",
    `fs.writeFileSync(${JSON.stringify(capturePath)}, JSON.stringify(payload));`,
    `process.stdout.write(${JSON.stringify(JSON.stringify(response))});`
  ].join("\n");
  await writeFile(scriptPath, script, "utf8");
}

async function deployWithCapture(input: {
  provider: ProviderAdapter;
  project: ProjectConfig;
  projectDir: string;
}): Promise<void> {
  const artifactPath = join(input.projectDir, "dist", "index.js");
  const artifactManifestPath = join(input.projectDir, "dist", "manifest.json");
  await mkdir(join(input.projectDir, "dist"), { recursive: true });
  await writeFile(artifactPath, "exports.handler = async () => ({ statusCode: 200 });\n", "utf8");
  await writeFile(artifactManifestPath, JSON.stringify({ provider: input.project.providers[0] }), "utf8");
  const plan = await input.provider.planDeploy(input.project, {
    provider: input.project.providers[0],
    entry: artifactPath,
    outputPath: artifactManifestPath
  });
  await input.provider.deploy(input.project, plan);
}

test("aws runtime mapping is forwarded to deploy command env", async () => {
  const mapping: Record<RuntimeFamily, string> = {
    nodejs: "nodejs20.x",
    python: "python3.12",
    go: "provided.al2023",
    java: "java21",
    rust: "provided.al2023",
    dotnet: "dotnet8"
  };

  for (const [runtime, expected] of Object.entries(mapping) as Array<[RuntimeFamily, string]>) {
    const projectDir = await mkdtemp(join(tmpdir(), "runfabric-aws-runtime-map-"));
    const capturePath = join(projectDir, "capture.json");
    const deployScriptPath = join(projectDir, "deploy.cjs");
    await writeCaptureScript(deployScriptPath, capturePath, {
      FunctionUrlConfig: { FunctionUrl: "https://runtime-map.aws/" }
    });

    await withEnv(
      {
        AWS_ACCESS_KEY_ID: "test",
        AWS_SECRET_ACCESS_KEY: "test",
        AWS_REGION: "us-east-1",
        RUNFABRIC_AWS_REAL_DEPLOY: "1",
        RUNFABRIC_AWS_DEPLOY_CMD: nodeScriptCommand(deployScriptPath)
      },
      async () => {
        const provider = createAwsLambdaProvider({ projectDir });
        await deployWithCapture({
          provider,
          project: createProject("aws-lambda", runtime),
          projectDir
        });
      }
    );

    const capture = JSON.parse(await readFile(capturePath, "utf8")) as {
      RUNFABRIC_AWS_RUNTIME: string;
    };
    assert.equal(capture.RUNFABRIC_AWS_RUNTIME, expected);
  }
});

test("gcp runtime mapping is forwarded to deploy command env", async () => {
  const mapping: Record<RuntimeFamily, string> = {
    nodejs: "nodejs20",
    python: "python312",
    go: "go122",
    java: "java21",
    rust: "nodejs20",
    dotnet: "dotnet8"
  };

  for (const [runtime, expected] of Object.entries(mapping) as Array<[RuntimeFamily, string]>) {
    const projectDir = await mkdtemp(join(tmpdir(), "runfabric-gcp-runtime-map-"));
    const capturePath = join(projectDir, "capture.json");
    const deployScriptPath = join(projectDir, "deploy.cjs");
    await writeCaptureScript(deployScriptPath, capturePath, {
      httpsTrigger: { url: "https://runtime-map.gcp/" }
    });

    await withEnv(
      {
        GCP_PROJECT_ID: "test-project",
        GCP_SERVICE_ACCOUNT_KEY: "{}",
        RUNFABRIC_GCP_REAL_DEPLOY: "1",
        RUNFABRIC_GCP_DEPLOY_CMD: nodeScriptCommand(deployScriptPath)
      },
      async () => {
        const provider = createGcpFunctionsProvider({ projectDir });
        await deployWithCapture({
          provider,
          project: createProject("gcp-functions", runtime),
          projectDir
        });
      }
    );

    const capture = JSON.parse(await readFile(capturePath, "utf8")) as {
      RUNFABRIC_GCP_RUNTIME: string;
    };
    assert.equal(capture.RUNFABRIC_GCP_RUNTIME, expected);
  }
});

test("ibm runtime kind mapping is forwarded to deploy command env", async () => {
  const mapping: Record<RuntimeFamily, string> = {
    nodejs: "nodejs:20",
    python: "python:3.11",
    go: "go:1.20",
    java: "java:17",
    rust: "blackbox",
    dotnet: "blackbox"
  };

  for (const [runtime, expected] of Object.entries(mapping) as Array<[RuntimeFamily, string]>) {
    const projectDir = await mkdtemp(join(tmpdir(), "runfabric-ibm-runtime-map-"));
    const capturePath = join(projectDir, "capture.json");
    const deployScriptPath = join(projectDir, "deploy.cjs");
    await writeCaptureScript(deployScriptPath, capturePath, {
      result: { endpoint: "https://runtime-map.ibm/" }
    });

    await withEnv(
      {
        IBM_CLOUD_API_KEY: "apikey",
        IBM_CLOUD_REGION: "us-south",
        IBM_CLOUD_NAMESPACE: "namespace",
        RUNFABRIC_IBM_REAL_DEPLOY: "1",
        RUNFABRIC_IBM_DEPLOY_CMD: nodeScriptCommand(deployScriptPath)
      },
      async () => {
        const provider = createIbmOpenWhiskProvider({ projectDir });
        await deployWithCapture({
          provider,
          project: createProject("ibm-openwhisk", runtime),
          projectDir
        });
      }
    );

    const capture = JSON.parse(await readFile(capturePath, "utf8")) as {
      RUNFABRIC_IBM_RUNTIME_KIND: string;
    };
    assert.equal(capture.RUNFABRIC_IBM_RUNTIME_KIND, expected);
  }
});
