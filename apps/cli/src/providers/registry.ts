import { createRequire } from "node:module";
import { join } from "node:path";
import type { ProviderAdapter } from "@runfabric/core";
import {
  buildProviderMetricsFromLocalArtifacts,
  buildProviderTracesFromLocalArtifacts,
  runProviderMetricsCommand,
  runProviderTracesCommand
} from "@runfabric/core";

interface ProviderModuleSpec {
  packageName: string;
  factoryExport: string;
}

type ProviderFactory = (options: { projectDir: string }) => ProviderAdapter;

const PROVIDER_MODULE_SPECS: Record<string, ProviderModuleSpec> = {
  "aws-lambda": {
    packageName: "@runfabric/provider-aws-lambda",
    factoryExport: "createAwsLambdaProvider"
  },
  "gcp-functions": {
    packageName: "@runfabric/provider-gcp-functions",
    factoryExport: "createGcpFunctionsProvider"
  },
  "azure-functions": {
    packageName: "@runfabric/provider-azure-functions",
    factoryExport: "createAzureFunctionsProvider"
  },
  kubernetes: {
    packageName: "@runfabric/provider-kubernetes",
    factoryExport: "createKubernetesProvider"
  },
  "cloudflare-workers": {
    packageName: "@runfabric/provider-cloudflare-workers",
    factoryExport: "createCloudflareWorkersProvider"
  },
  vercel: {
    packageName: "@runfabric/provider-vercel",
    factoryExport: "createVercelProvider"
  },
  netlify: {
    packageName: "@runfabric/provider-netlify",
    factoryExport: "createNetlifyProvider"
  },
  "alibaba-fc": {
    packageName: "@runfabric/provider-alibaba-fc",
    factoryExport: "createAlibabaFcProvider"
  },
  "digitalocean-functions": {
    packageName: "@runfabric/provider-digitalocean-functions",
    factoryExport: "createDigitalOceanFunctionsProvider"
  },
  "fly-machines": {
    packageName: "@runfabric/provider-fly-machines",
    factoryExport: "createFlyMachinesProvider"
  },
  "ibm-openwhisk": {
    packageName: "@runfabric/provider-ibm-openwhisk",
    factoryExport: "createIbmOpenWhiskProvider"
  }
};

export const KNOWN_PROVIDER_IDS = Object.keys(PROVIDER_MODULE_SPECS);

const OBSERVABILITY_ENV_PREFIX: Record<string, string> = {
  "aws-lambda": "AWS",
  "gcp-functions": "GCP",
  "azure-functions": "AZURE",
  kubernetes: "KUBERNETES",
  "cloudflare-workers": "CLOUDFLARE",
  vercel: "VERCEL",
  netlify: "NETLIFY",
  "alibaba-fc": "ALIBABA",
  "digitalocean-functions": "DIGITALOCEAN",
  "fly-machines": "FLY",
  "ibm-openwhisk": "IBM"
};

function isModuleNotFoundError(error: unknown, packageName: string): boolean {
  if (!error || typeof error !== "object") {
    return false;
  }

  const candidate = error as { code?: string; message?: string };
  if (candidate.code !== "MODULE_NOT_FOUND") {
    return false;
  }

  const message = typeof candidate.message === "string" ? candidate.message : "";
  return message.includes(`'${packageName}'`) || message.includes(`"${packageName}"`);
}

function createProjectRequire(projectDir: string): NodeJS.Require | undefined {
  try {
    return createRequire(join(projectDir, "package.json"));
  } catch {
    return undefined;
  }
}

function loadProviderModule(
  spec: ProviderModuleSpec,
  projectDir: string
): Record<string, unknown> | undefined {
  const projectRequire = createProjectRequire(projectDir);
  const requireChain: NodeJS.Require[] = [];
  if (projectRequire) {
    requireChain.push(projectRequire);
  }
  requireChain.push(require);

  for (const loader of requireChain) {
    try {
      return loader(spec.packageName) as Record<string, unknown>;
    } catch (error) {
      if (isModuleNotFoundError(error, spec.packageName)) {
        continue;
      }
      throw error;
    }
  }

  return undefined;
}

function loadProviderFactory(provider: string, projectDir: string): ProviderFactory | undefined {
  const spec = PROVIDER_MODULE_SPECS[provider];
  if (!spec) {
    return undefined;
  }

  const providerModule = loadProviderModule(spec, projectDir);
  if (!providerModule) {
    return undefined;
  }

  const factory = providerModule[spec.factoryExport];
  if (typeof factory !== "function") {
    throw new Error(
      `${spec.packageName} is installed but does not export ${spec.factoryExport}; upgrade the provider package`
    );
  }
  return factory as ProviderFactory;
}

export function getProviderPackageName(provider: string): string | undefined {
  return PROVIDER_MODULE_SPECS[provider]?.packageName;
}

function envCommand(envName: string | undefined): string | undefined {
  if (!envName) {
    return undefined;
  }
  const rawValue = process.env[envName];
  if (!rawValue || rawValue.trim().length === 0) {
    return undefined;
  }
  return rawValue;
}

function withObservability(adapter: ProviderAdapter, projectDir: string): ProviderAdapter {
  const envPrefix = OBSERVABILITY_ENV_PREFIX[adapter.name];
  const tracesEnvName = envPrefix ? `RUNFABRIC_${envPrefix}_TRACES_CMD` : undefined;
  const metricsEnvName = envPrefix ? `RUNFABRIC_${envPrefix}_METRICS_CMD` : undefined;
  const nativeTraces = adapter.traces?.bind(adapter);
  const nativeMetrics = adapter.metrics?.bind(adapter);

  adapter.traces = async (input) => {
    const command = envCommand(tracesEnvName);
    if (command) {
      return runProviderTracesCommand(command, adapter.name, { cwd: projectDir });
    }
    if (nativeTraces) {
      return nativeTraces(input);
    }
    return buildProviderTracesFromLocalArtifacts(projectDir, adapter.name, input);
  };

  adapter.metrics = async (input) => {
    const command = envCommand(metricsEnvName);
    if (command) {
      return runProviderMetricsCommand(command, { cwd: projectDir });
    }
    if (nativeMetrics) {
      return nativeMetrics(input);
    }
    return buildProviderMetricsFromLocalArtifacts(projectDir, adapter.name, input);
  };

  return adapter;
}

export function createProviderRegistry(
  projectDir: string,
  targetProviders: string[] = KNOWN_PROVIDER_IDS
): Record<string, ProviderAdapter> {
  const registry: Record<string, ProviderAdapter> = {};

  for (const provider of targetProviders) {
    const factory = loadProviderFactory(provider, projectDir);
    if (!factory) {
      continue;
    }
    registry[provider] = withObservability(factory({ projectDir }), projectDir);
  }

  return registry;
}
