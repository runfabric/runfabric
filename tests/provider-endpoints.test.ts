import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import type { ProviderAdapter, ProjectConfig } from "../packages/core/src/index.ts";
import { TriggerEnum } from "../packages/core/src/index.ts";
import { createAlibabaFcProvider } from "../packages/provider-alibaba-fc/src/index.ts";
import { createAzureFunctionsProvider } from "../packages/provider-azure-functions/src/index.ts";
import { createCloudflareWorkersProvider } from "../packages/provider-cloudflare-workers/src/index.ts";
import { createDigitalOceanFunctionsProvider } from "../packages/provider-digitalocean-functions/src/index.ts";
import { createFlyMachinesProvider } from "../packages/provider-fly-machines/src/index.ts";
import { createGcpFunctionsProvider } from "../packages/provider-gcp-functions/src/index.ts";
import { createIbmOpenWhiskProvider } from "../packages/provider-ibm-openwhisk/src/index.ts";
import { createKubernetesProvider } from "../packages/provider-kubernetes/src/index.ts";
import { createNetlifyProvider } from "../packages/provider-netlify/src/index.ts";
import { createVercelProvider } from "../packages/provider-vercel/src/index.ts";

interface ProviderEndpointCase {
  provider: string;
  endpointIncludes: string;
  createProvider(projectDir: string): ProviderAdapter;
}

function createProject(provider: string): ProjectConfig {
  return {
    service: "hello-http",
    runtime: "nodejs",
    entry: "src/index.ts",
    providers: [provider],
    triggers: [{ type: TriggerEnum.Http, method: "GET", path: "/hello" }]
  };
}

const cases: ProviderEndpointCase[] = [
  {
    provider: "cloudflare-workers",
    endpointIncludes: ".workers.dev",
    createProvider: (projectDir) => createCloudflareWorkersProvider({ projectDir })
  },
  {
    provider: "vercel",
    endpointIncludes: ".vercel.app",
    createProvider: (projectDir) => createVercelProvider({ projectDir })
  },
  {
    provider: "netlify",
    endpointIncludes: ".netlify.app",
    createProvider: (projectDir) => createNetlifyProvider({ projectDir })
  },
  {
    provider: "gcp-functions",
    endpointIncludes: ".cloudfunctions.net/",
    createProvider: (projectDir) => createGcpFunctionsProvider({ projectDir })
  },
  {
    provider: "azure-functions",
    endpointIncludes: ".azurewebsites.net",
    createProvider: (projectDir) => createAzureFunctionsProvider({ projectDir })
  },
  {
    provider: "kubernetes",
    endpointIncludes: ".svc.cluster.local",
    createProvider: (projectDir) => createKubernetesProvider({ projectDir })
  },
  {
    provider: "alibaba-fc",
    endpointIncludes: ".fcapp.run",
    createProvider: (projectDir) => createAlibabaFcProvider({ projectDir })
  },
  {
    provider: "digitalocean-functions",
    endpointIncludes: ".doserverless.co/",
    createProvider: (projectDir) => createDigitalOceanFunctionsProvider({ projectDir })
  },
  {
    provider: "fly-machines",
    endpointIncludes: ".fly.dev",
    createProvider: (projectDir) => createFlyMachinesProvider({ projectDir })
  },
  {
    provider: "ibm-openwhisk",
    endpointIncludes: ".functions.cloud.ibm.com/",
    createProvider: (projectDir) => createIbmOpenWhiskProvider({ projectDir })
  }
];

for (const testCase of cases) {
  test(`provider endpoint output: ${testCase.provider}`, async () => {
    const projectDir = await mkdtemp(join(tmpdir(), "runfabric-provider-endpoint-"));
    const project = createProject(testCase.provider);
    const provider = testCase.createProvider(projectDir);
    const artifact = {
      provider: testCase.provider,
      entry: "src/index.ts",
      outputPath: "tmp/artifact.json"
    };

    const deployPlan = await provider.planDeploy(project, artifact);
    const deployResult = await provider.deploy(project, deployPlan);
    assert.ok(
      deployResult.endpoint?.includes(testCase.endpointIncludes),
      `${testCase.provider} endpoint should include ${testCase.endpointIncludes}, got ${deployResult.endpoint}`
    );
  });
}
