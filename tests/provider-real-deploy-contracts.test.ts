import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { ProjectConfig, ProviderAdapter } from "../packages/core/src/index.ts";
import { TriggerEnum } from "../packages/core/src/index.ts";
import { createAlibabaFcProvider } from "../packages/provider-alibaba-fc/src/index.ts";
import { createAwsLambdaProvider } from "../packages/provider-aws-lambda/src/index.ts";
import { createAzureFunctionsProvider } from "../packages/provider-azure-functions/src/index.ts";
import { createDigitalOceanFunctionsProvider } from "../packages/provider-digitalocean-functions/src/index.ts";
import { createFlyMachinesProvider } from "../packages/provider-fly-machines/src/index.ts";
import { createGcpFunctionsProvider } from "../packages/provider-gcp-functions/src/index.ts";
import { createIbmOpenWhiskProvider } from "../packages/provider-ibm-openwhisk/src/index.ts";
import { createKubernetesProvider } from "../packages/provider-kubernetes/src/index.ts";
import { createNetlifyProvider } from "../packages/provider-netlify/src/index.ts";
import { createVercelProvider } from "../packages/provider-vercel/src/index.ts";

interface ProviderRealDeployCase {
  provider: string;
  enableRealDeployEnv: string;
  deployCommandEnv: string;
  destroyCommandEnv: string;
  requiredCredentials: Record<string, string>;
  deployResponseFixture: Record<string, unknown>;
  expectedEndpoint: string;
  expectedAwsEnv?: Record<string, string>;
  createProvider(projectDir: string): ProviderAdapter;
}

const providerCases: ProviderRealDeployCase[] = [
  {
    provider: "aws-lambda",
    enableRealDeployEnv: "RUNFABRIC_AWS_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_AWS_DEPLOY_CMD",
    destroyCommandEnv: "RUNFABRIC_AWS_DESTROY_CMD",
    requiredCredentials: {
      AWS_ACCESS_KEY_ID: "test",
      AWS_SECRET_ACCESS_KEY: "test",
      AWS_REGION: "us-east-1"
    },
    deployResponseFixture: {
      FunctionUrlConfig: {
        FunctionUrl: "https://aws-contract.lambda-url.us-east-1.on.aws/"
      }
    },
    expectedEndpoint: "https://aws-contract.lambda-url.us-east-1.on.aws/",
    expectedAwsEnv: {
      RUNFABRIC_AWS_QUEUE_EVENT_SOURCES_JSON: "[]",
      RUNFABRIC_AWS_STORAGE_EVENTS_JSON: "[]",
      RUNFABRIC_AWS_EVENTBRIDGE_RULES_JSON: "[]",
      RUNFABRIC_AWS_IAM_ROLE_STATEMENTS_JSON: "[]",
      RUNFABRIC_FUNCTION_ENV_JSON: "{}",
      RUNFABRIC_AWS_RESOURCE_ADDRESSES_JSON: "{}",
      RUNFABRIC_AWS_WORKFLOW_ADDRESSES_JSON: "{}",
      RUNFABRIC_AWS_SECRET_REFERENCES_JSON: "{}"
    },
    createProvider: (projectDir) => createAwsLambdaProvider({ projectDir })
  },
  {
    provider: "gcp-functions",
    enableRealDeployEnv: "RUNFABRIC_GCP_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_GCP_DEPLOY_CMD",
    destroyCommandEnv: "RUNFABRIC_GCP_DESTROY_CMD",
    requiredCredentials: {
      GCP_PROJECT_ID: "test-project",
      GCP_SERVICE_ACCOUNT_KEY: "{}"
    },
    deployResponseFixture: {
      httpsTrigger: {
        url: "https://us-central1-test-project.cloudfunctions.net/contract-gcp"
      }
    },
    expectedEndpoint: "https://us-central1-test-project.cloudfunctions.net/contract-gcp",
    createProvider: (projectDir) => createGcpFunctionsProvider({ projectDir })
  },
  {
    provider: "azure-functions",
    enableRealDeployEnv: "RUNFABRIC_AZURE_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_AZURE_DEPLOY_CMD",
    destroyCommandEnv: "RUNFABRIC_AZURE_DESTROY_CMD",
    requiredCredentials: {
      AZURE_TENANT_ID: "tenant",
      AZURE_CLIENT_ID: "client",
      AZURE_CLIENT_SECRET: "secret",
      AZURE_SUBSCRIPTION_ID: "subscription",
      AZURE_RESOURCE_GROUP: "rg"
    },
    deployResponseFixture: {
      properties: {
        defaultHostName: "contract-azure.azurewebsites.net"
      }
    },
    expectedEndpoint: "https://contract-azure.azurewebsites.net",
    createProvider: (projectDir) => createAzureFunctionsProvider({ projectDir })
  },
  {
    provider: "kubernetes",
    enableRealDeployEnv: "RUNFABRIC_KUBERNETES_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_KUBERNETES_DEPLOY_CMD",
    destroyCommandEnv: "RUNFABRIC_KUBERNETES_DESTROY_CMD",
    requiredCredentials: {
      KUBECONFIG: "/tmp/kubeconfig"
    },
    deployResponseFixture: {
      endpoint: "https://contract-k8s.default.svc.cluster.local"
    },
    expectedEndpoint: "https://contract-k8s.default.svc.cluster.local",
    createProvider: (projectDir) => createKubernetesProvider({ projectDir })
  },
  {
    provider: "vercel",
    enableRealDeployEnv: "RUNFABRIC_VERCEL_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_VERCEL_DEPLOY_CMD",
    destroyCommandEnv: "RUNFABRIC_VERCEL_DESTROY_CMD",
    requiredCredentials: {
      VERCEL_TOKEN: "token",
      VERCEL_ORG_ID: "org",
      VERCEL_PROJECT_ID: "project"
    },
    deployResponseFixture: {
      alias: ["contract-vercel.vercel.app"]
    },
    expectedEndpoint: "https://contract-vercel.vercel.app",
    createProvider: (projectDir) => createVercelProvider({ projectDir })
  },
  {
    provider: "netlify",
    enableRealDeployEnv: "RUNFABRIC_NETLIFY_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_NETLIFY_DEPLOY_CMD",
    destroyCommandEnv: "RUNFABRIC_NETLIFY_DESTROY_CMD",
    requiredCredentials: {
      NETLIFY_AUTH_TOKEN: "token",
      NETLIFY_SITE_ID: "site"
    },
    deployResponseFixture: {
      published_deploy: {
        url: "https://contract-netlify.netlify.app"
      }
    },
    expectedEndpoint: "https://contract-netlify.netlify.app",
    createProvider: (projectDir) => createNetlifyProvider({ projectDir })
  },
  {
    provider: "alibaba-fc",
    enableRealDeployEnv: "RUNFABRIC_ALIBABA_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_ALIBABA_DEPLOY_CMD",
    destroyCommandEnv: "RUNFABRIC_ALIBABA_DESTROY_CMD",
    requiredCredentials: {
      ALICLOUD_ACCESS_KEY_ID: "key",
      ALICLOUD_ACCESS_KEY_SECRET: "secret",
      ALICLOUD_REGION: "cn-hangzhou"
    },
    deployResponseFixture: {
      result: {
        endpoint: "contract-alibaba.cn-hangzhou.fcapp.run"
      }
    },
    expectedEndpoint: "https://contract-alibaba.cn-hangzhou.fcapp.run",
    createProvider: (projectDir) => createAlibabaFcProvider({ projectDir })
  },
  {
    provider: "digitalocean-functions",
    enableRealDeployEnv: "RUNFABRIC_DIGITALOCEAN_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_DIGITALOCEAN_DEPLOY_CMD",
    destroyCommandEnv: "RUNFABRIC_DIGITALOCEAN_DESTROY_CMD",
    requiredCredentials: {
      DIGITALOCEAN_ACCESS_TOKEN: "token",
      DIGITALOCEAN_NAMESPACE: "namespace"
    },
    deployResponseFixture: {
      result: {
        endpoint: "https://faas-nyc1.doserverless.co/api/v1/web/namespace/default/contract"
      }
    },
    expectedEndpoint: "https://faas-nyc1.doserverless.co/api/v1/web/namespace/default/contract",
    createProvider: (projectDir) => createDigitalOceanFunctionsProvider({ projectDir })
  },
  {
    provider: "fly-machines",
    enableRealDeployEnv: "RUNFABRIC_FLY_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_FLY_DEPLOY_CMD",
    destroyCommandEnv: "RUNFABRIC_FLY_DESTROY_CMD",
    requiredCredentials: {
      FLY_API_TOKEN: "token",
      FLY_APP_NAME: "contract-fly"
    },
    deployResponseFixture: {
      app: {
        name: "contract-fly-app"
      }
    },
    expectedEndpoint: "https://contract-fly-app.fly.dev",
    createProvider: (projectDir) => createFlyMachinesProvider({ projectDir })
  },
  {
    provider: "ibm-openwhisk",
    enableRealDeployEnv: "RUNFABRIC_IBM_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_IBM_DEPLOY_CMD",
    destroyCommandEnv: "RUNFABRIC_IBM_DESTROY_CMD",
    requiredCredentials: {
      IBM_CLOUD_API_KEY: "apikey",
      IBM_CLOUD_REGION: "us-south",
      IBM_CLOUD_NAMESPACE: "namespace"
    },
    deployResponseFixture: {
      result: {
        endpoint: "us-south.functions.cloud.ibm.com/api/v1/web/namespace/default/contract"
      }
    },
    expectedEndpoint: "https://us-south.functions.cloud.ibm.com/api/v1/web/namespace/default/contract",
    createProvider: (projectDir) => createIbmOpenWhiskProvider({ projectDir })
  }
];

function createProject(provider: string): ProjectConfig {
  return {
    service: `contract-${provider.replace(/[^a-z0-9-]/gi, "-")}`,
    runtime: "nodejs",
    entry: "src/index.ts",
    stage: "ci",
    providers: [provider],
    triggers: [{ type: TriggerEnum.Http, method: "GET", path: "/contract" }]
  };
}

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

function nodeScriptCommand(scriptPath: string): string {
  return `node ${JSON.stringify(scriptPath)}`;
}

function expectedDeployEnv(project: ProjectConfig, artifactPath: string, artifactManifestPath: string): Record<string, string> {
  return {
    RUNFABRIC_SERVICE: project.service,
    RUNFABRIC_STAGE: project.stage || "default",
    RUNFABRIC_ARTIFACT_PATH: artifactPath,
    RUNFABRIC_ARTIFACT_MANIFEST_PATH: artifactManifestPath
  };
}

function expectedDestroyEnv(project: ProjectConfig): Record<string, string> {
  return {
    RUNFABRIC_SERVICE: project.service,
    RUNFABRIC_STAGE: project.stage || "default"
  };
}

function captureEnvScript(filePath: string, capturePath: string, response?: unknown): Promise<void> {
  const script = [
    "const fs = require('node:fs');",
    "const payload = {",
    "  RUNFABRIC_SERVICE: process.env.RUNFABRIC_SERVICE || '',",
    "  RUNFABRIC_STAGE: process.env.RUNFABRIC_STAGE || '',",
    "  RUNFABRIC_ARTIFACT_PATH: process.env.RUNFABRIC_ARTIFACT_PATH || '',",
    "  RUNFABRIC_ARTIFACT_MANIFEST_PATH: process.env.RUNFABRIC_ARTIFACT_MANIFEST_PATH || '',",
    "  RUNFABRIC_AWS_QUEUE_EVENT_SOURCES_JSON: process.env.RUNFABRIC_AWS_QUEUE_EVENT_SOURCES_JSON || '',",
    "  RUNFABRIC_AWS_STORAGE_EVENTS_JSON: process.env.RUNFABRIC_AWS_STORAGE_EVENTS_JSON || '',",
    "  RUNFABRIC_AWS_EVENTBRIDGE_RULES_JSON: process.env.RUNFABRIC_AWS_EVENTBRIDGE_RULES_JSON || '',",
    "  RUNFABRIC_AWS_IAM_ROLE_STATEMENTS_JSON: process.env.RUNFABRIC_AWS_IAM_ROLE_STATEMENTS_JSON || '',",
    "  RUNFABRIC_FUNCTION_ENV_JSON: process.env.RUNFABRIC_FUNCTION_ENV_JSON || '',",
    "  RUNFABRIC_AWS_RESOURCE_ADDRESSES_JSON: process.env.RUNFABRIC_AWS_RESOURCE_ADDRESSES_JSON || '',",
    "  RUNFABRIC_AWS_WORKFLOW_ADDRESSES_JSON: process.env.RUNFABRIC_AWS_WORKFLOW_ADDRESSES_JSON || '',",
    "  RUNFABRIC_AWS_SECRET_REFERENCES_JSON: process.env.RUNFABRIC_AWS_SECRET_REFERENCES_JSON || ''",
    "};",
    `fs.writeFileSync(${JSON.stringify(capturePath)}, JSON.stringify(payload));`,
    response !== undefined ? `process.stdout.write(${JSON.stringify(JSON.stringify(response))});` : "process.stdout.write('{}');"
  ].join("\n");
  return writeFile(filePath, script, "utf8");
}

for (const testCase of providerCases) {
  test(`real deploy command contract: ${testCase.provider}`, async () => {
    const projectDir = await mkdtemp(join(tmpdir(), "runfabric-provider-contract-"));
    const project = createProject(testCase.provider);
    const provider = testCase.createProvider(projectDir);
    const artifactPath = join(projectDir, "dist", "index.js");
    const artifactManifestPath = join(projectDir, "dist", "manifest.json");

    await mkdir(join(projectDir, "src"), { recursive: true });
    await mkdir(join(projectDir, "dist"), { recursive: true });
    await writeFile(join(projectDir, "src", "index.ts"), "export const handler = async () => ({ statusCode: 200 });\n", "utf8");
    await writeFile(artifactPath, "exports.handler = async () => ({ statusCode: 200 });\n", "utf8");
    await writeFile(artifactManifestPath, JSON.stringify({ provider: testCase.provider }), "utf8");

    const deployCapturePath = join(projectDir, `${testCase.provider}-deploy-env.json`);
    const destroyCapturePath = join(projectDir, `${testCase.provider}-destroy-env.json`);
    const deployScriptPath = join(projectDir, `${testCase.provider}-deploy.cjs`);
    const destroyScriptPath = join(projectDir, `${testCase.provider}-destroy.cjs`);

    await captureEnvScript(deployScriptPath, deployCapturePath, testCase.deployResponseFixture);
    await captureEnvScript(destroyScriptPath, destroyCapturePath);

    await withEnv(
      {
        ...testCase.requiredCredentials,
        [testCase.enableRealDeployEnv]: "1",
        [testCase.deployCommandEnv]: nodeScriptCommand(deployScriptPath),
        [testCase.destroyCommandEnv]: nodeScriptCommand(destroyScriptPath)
      },
      async () => {
        const deployPlan = await provider.planDeploy(project, {
          provider: testCase.provider,
          entry: artifactPath,
          outputPath: artifactManifestPath
        });
        const deployResult = await provider.deploy(project, deployPlan);
        assert.equal(deployResult.endpoint, testCase.expectedEndpoint);

        const deployCaptured = JSON.parse(await readFile(deployCapturePath, "utf8")) as Record<string, string>;
        const expectedDeploy = expectedDeployEnv(project, artifactPath, artifactManifestPath);
        for (const [key, value] of Object.entries(expectedDeploy)) {
          assert.equal(deployCaptured[key], value, `${testCase.provider} deploy env ${key} should match`);
        }
        if (testCase.expectedAwsEnv) {
          for (const [key, value] of Object.entries(testCase.expectedAwsEnv)) {
            assert.equal(deployCaptured[key], value, `${testCase.provider} deploy env ${key} should match`);
          }
        }

        const receiptPath = join(projectDir, ".runfabric", "deploy", testCase.provider, "deployment.json");
        const receipt = JSON.parse(await readFile(receiptPath, "utf8")) as { mode?: string };
        assert.equal(receipt.mode, "cli");

        await provider.destroy?.(project);
        const destroyCaptured = JSON.parse(await readFile(destroyCapturePath, "utf8")) as Record<string, string>;
        const expectedDestroy = expectedDestroyEnv(project);
        for (const [key, value] of Object.entries(expectedDestroy)) {
          assert.equal(destroyCaptured[key], value, `${testCase.provider} destroy env ${key} should match`);
        }
      }
    );
  });

  test(`real deploy invalid JSON is handled: ${testCase.provider}`, async () => {
    const projectDir = await mkdtemp(join(tmpdir(), "runfabric-provider-invalid-json-"));
    const project = createProject(testCase.provider);
    const provider = testCase.createProvider(projectDir);
    const invalidScriptPath = join(projectDir, `${testCase.provider}-invalid.cjs`);

    await writeFile(invalidScriptPath, "process.stdout.write('not-json');\n", "utf8");

    await withEnv(
      {
        ...testCase.requiredCredentials,
        [testCase.enableRealDeployEnv]: "1",
        [testCase.deployCommandEnv]: nodeScriptCommand(invalidScriptPath)
      },
      async () => {
        const deployPlan = await provider.planDeploy(project, {
          provider: testCase.provider,
          entry: "dist/index.js",
          outputPath: "dist/manifest.json"
        });

        await assert.rejects(
          provider.deploy(project, deployPlan),
          /command output is not valid JSON/,
          `${testCase.provider} should fail with invalid JSON error`
        );
      }
    );
  });
}
