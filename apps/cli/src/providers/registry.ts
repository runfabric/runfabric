import type { ProviderAdapter } from "@runfabric/core";
import { createAwsLambdaProvider } from "@runfabric/provider-aws-lambda";
import { createAlibabaFcProvider } from "@runfabric/provider-alibaba-fc";
import { createAzureFunctionsProvider } from "@runfabric/provider-azure-functions";
import { createCloudflareWorkersProvider } from "@runfabric/provider-cloudflare-workers";
import { createDigitalOceanFunctionsProvider } from "@runfabric/provider-digitalocean-functions";
import { createFlyMachinesProvider } from "@runfabric/provider-fly-machines";
import { createGcpFunctionsProvider } from "@runfabric/provider-gcp-functions";
import { createIbmOpenWhiskProvider } from "@runfabric/provider-ibm-openwhisk";
import { createNetlifyProvider } from "@runfabric/provider-netlify";
import { createVercelProvider } from "@runfabric/provider-vercel";

export function createProviderRegistry(projectDir: string): Record<string, ProviderAdapter> {
  return {
    "aws-lambda": createAwsLambdaProvider({ projectDir }),
    "gcp-functions": createGcpFunctionsProvider({ projectDir }),
    "azure-functions": createAzureFunctionsProvider({ projectDir }),
    "cloudflare-workers": createCloudflareWorkersProvider({ projectDir }),
    vercel: createVercelProvider({ projectDir }),
    netlify: createNetlifyProvider({ projectDir }),
    "alibaba-fc": createAlibabaFcProvider({ projectDir }),
    "digitalocean-functions": createDigitalOceanFunctionsProvider({ projectDir }),
    "fly-machines": createFlyMachinesProvider({ projectDir }),
    "ibm-openwhisk": createIbmOpenWhiskProvider({ projectDir })
  };
}
