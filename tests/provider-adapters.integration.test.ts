import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import type { ProjectConfig, ProviderAdapter } from "../packages/core/src/index.ts";
import { TriggerEnum } from "../packages/core/src/index.ts";
import { createPlan } from "../packages/planner/src/planner.ts";
import { buildProject } from "../packages/builder/src/index.ts";
import { createAlibabaFcProvider } from "../packages/provider-alibaba-fc/src/index.ts";
import { createAwsLambdaProvider } from "../packages/provider-aws-lambda/src/index.ts";
import { createAzureFunctionsProvider } from "../packages/provider-azure-functions/src/index.ts";
import { createCloudflareWorkersProvider } from "../packages/provider-cloudflare-workers/src/index.ts";
import { createDigitalOceanFunctionsProvider } from "../packages/provider-digitalocean-functions/src/index.ts";
import { createFlyMachinesProvider } from "../packages/provider-fly-machines/src/index.ts";
import { createGcpFunctionsProvider } from "../packages/provider-gcp-functions/src/index.ts";
import { createIbmOpenWhiskProvider } from "../packages/provider-ibm-openwhisk/src/index.ts";
import { createKubernetesProvider } from "../packages/provider-kubernetes/src/index.ts";
import { createNetlifyProvider } from "../packages/provider-netlify/src/index.ts";
import { createVercelProvider } from "../packages/provider-vercel/src/index.ts";

interface ProviderIntegrationCase {
  provider: string;
  endpointIncludes: string;
  env: Record<string, string>;
  createProvider(projectDir: string): ProviderAdapter;
}

const providerCases: ProviderIntegrationCase[] = [
  {
    provider: "aws-lambda",
    endpointIncludes: ".execute-api.",
    env: {
      AWS_ACCESS_KEY_ID: "test",
      AWS_SECRET_ACCESS_KEY: "test",
      AWS_REGION: "us-east-1"
    },
    createProvider: (projectDir) => createAwsLambdaProvider({ projectDir })
  },
  {
    provider: "gcp-functions",
    endpointIncludes: ".cloudfunctions.net/",
    env: {
      GCP_PROJECT_ID: "test-project",
      GCP_SERVICE_ACCOUNT_KEY: "{}"
    },
    createProvider: (projectDir) => createGcpFunctionsProvider({ projectDir })
  },
  {
    provider: "azure-functions",
    endpointIncludes: ".azurewebsites.net",
    env: {
      AZURE_TENANT_ID: "tenant",
      AZURE_CLIENT_ID: "client",
      AZURE_CLIENT_SECRET: "secret",
      AZURE_SUBSCRIPTION_ID: "subscription",
      AZURE_RESOURCE_GROUP: "rg"
    },
    createProvider: (projectDir) => createAzureFunctionsProvider({ projectDir })
  },
  {
    provider: "kubernetes",
    endpointIncludes: ".svc.cluster.local",
    env: {
      KUBECONFIG: "/tmp/kubeconfig"
    },
    createProvider: (projectDir) => createKubernetesProvider({ projectDir })
  },
  {
    provider: "cloudflare-workers",
    endpointIncludes: ".workers.dev",
    env: {
      CLOUDFLARE_API_TOKEN: "token",
      CLOUDFLARE_ACCOUNT_ID: "account",
      RUNFABRIC_CLOUDFLARE_REAL_DEPLOY: "0"
    },
    createProvider: (projectDir) => createCloudflareWorkersProvider({ projectDir })
  },
  {
    provider: "vercel",
    endpointIncludes: ".vercel.app",
    env: {
      VERCEL_TOKEN: "token",
      VERCEL_ORG_ID: "org",
      VERCEL_PROJECT_ID: "project"
    },
    createProvider: (projectDir) => createVercelProvider({ projectDir })
  },
  {
    provider: "netlify",
    endpointIncludes: ".netlify.app",
    env: {
      NETLIFY_AUTH_TOKEN: "token",
      NETLIFY_SITE_ID: "site"
    },
    createProvider: (projectDir) => createNetlifyProvider({ projectDir })
  },
  {
    provider: "alibaba-fc",
    endpointIncludes: ".fcapp.run",
    env: {
      ALICLOUD_ACCESS_KEY_ID: "key",
      ALICLOUD_ACCESS_KEY_SECRET: "secret",
      ALICLOUD_REGION: "cn-hangzhou"
    },
    createProvider: (projectDir) => createAlibabaFcProvider({ projectDir })
  },
  {
    provider: "digitalocean-functions",
    endpointIncludes: ".doserverless.co/",
    env: {
      DIGITALOCEAN_ACCESS_TOKEN: "token",
      DIGITALOCEAN_NAMESPACE: "namespace"
    },
    createProvider: (projectDir) => createDigitalOceanFunctionsProvider({ projectDir })
  },
  {
    provider: "fly-machines",
    endpointIncludes: ".fly.dev",
    env: {
      FLY_API_TOKEN: "token",
      FLY_APP_NAME: "app"
    },
    createProvider: (projectDir) => createFlyMachinesProvider({ projectDir })
  },
  {
    provider: "ibm-openwhisk",
    endpointIncludes: ".functions.cloud.ibm.com/",
    env: {
      IBM_CLOUD_API_KEY: "apikey",
      IBM_CLOUD_REGION: "us-south",
      IBM_CLOUD_NAMESPACE: "namespace"
    },
    createProvider: (projectDir) => createIbmOpenWhiskProvider({ projectDir })
  }
];

function createProject(provider: string): ProjectConfig {
  return {
    service: `integration-${provider.replace(/[^a-z0-9-]/gi, "-")}`,
    runtime: "nodejs",
    entry: "src/index.ts",
    providers: [provider],
    triggers: [{ type: TriggerEnum.Http, method: "GET", path: "/integration" }]
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

for (const testCase of providerCases) {
  test(`provider integration path: ${testCase.provider}`, async () => {
    await withEnv(testCase.env, async () => {
      const projectDir = await mkdtemp(join(tmpdir(), "runfabric-provider-int-"));
      await mkdir(join(projectDir, "src"), { recursive: true });
      await writeFile(
        join(projectDir, "src", "index.ts"),
        "export const handler = async () => ({ statusCode: 200, body: 'ok' });\n",
        "utf8"
      );

      const project = createProject(testCase.provider);
      const provider = testCase.createProvider(projectDir);

      const validation = await provider.validate(project);
      assert.equal(validation.ok, true, `${testCase.provider} validation should pass`);

      const providerBuildPlan = await provider.planBuild(project);
      await provider.build(project, providerBuildPlan);

      const planning = createPlan(project);
      assert.equal(planning.ok, true, `${testCase.provider} planning should pass`);

      const build = await buildProject({
        planning,
        project,
        projectDir
      });
      const artifact = build.artifacts.find((item) => item.provider === testCase.provider);
      assert.ok(artifact, `${testCase.provider} artifact should be produced`);

      const deployPlan = await provider.planDeploy(project, artifact!);
      const deploy = await provider.deploy(project, deployPlan);
      assert.equal(deploy.provider, testCase.provider);
      assert.ok(
        deploy.endpoint?.includes(testCase.endpointIncludes),
        `${testCase.provider} endpoint should include ${testCase.endpointIncludes}`
      );
    });
  });
}
