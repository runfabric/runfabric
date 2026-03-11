import { createRequire } from "node:module";
import { join } from "node:path";
import type { ProviderAdapter } from "@runfabric/core";
import {
  buildProviderMetricsFromLocalArtifacts,
  buildProviderTracesFromLocalArtifacts,
  runJsonCommand,
  type MetricsResult,
  type TraceRecord,
  type TracesResult
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

function withObservability(adapter: ProviderAdapter, projectDir: string): ProviderAdapter {
  const envPrefix = OBSERVABILITY_ENV_PREFIX[adapter.name];
  const tracesEnvName = envPrefix ? `RUNFABRIC_${envPrefix}_TRACES_CMD` : undefined;
  const metricsEnvName = envPrefix ? `RUNFABRIC_${envPrefix}_METRICS_CMD` : undefined;
  const nativeTraces = adapter.traces?.bind(adapter);
  const nativeMetrics = adapter.metrics?.bind(adapter);

  const parseTraces = (raw: unknown): TracesResult => {
    if (Array.isArray(raw)) {
      const traces = raw
        .filter((entry): entry is Record<string, unknown> => Boolean(entry) && typeof entry === "object")
        .map((entry) => {
          const trace: TraceRecord = {
            timestamp:
              typeof entry.timestamp === "string" && entry.timestamp.trim().length > 0
                ? entry.timestamp
                : new Date().toISOString(),
            provider: adapter.name,
            message:
              typeof entry.message === "string" && entry.message.trim().length > 0
                ? entry.message
                : JSON.stringify(entry),
            deploymentId:
              typeof entry.deploymentId === "string" && entry.deploymentId.trim().length > 0
                ? entry.deploymentId
                : undefined,
            invokeId:
              typeof entry.invokeId === "string" && entry.invokeId.trim().length > 0
                ? entry.invokeId
                : undefined,
            correlationId:
              typeof entry.correlationId === "string" && entry.correlationId.trim().length > 0
                ? entry.correlationId
                : undefined
          };
          return trace;
        });
      return { traces };
    }

    if (raw && typeof raw === "object") {
      const payload = raw as Record<string, unknown>;
      if (Array.isArray(payload.traces)) {
        return parseTraces(payload.traces);
      }
    }

    throw new Error("trace command output must be { traces: [...] } or an array");
  };

  const parseMetrics = (raw: unknown): MetricsResult => {
    if (Array.isArray(raw)) {
      const metrics = raw
        .filter((entry): entry is Record<string, unknown> => Boolean(entry) && typeof entry === "object")
        .map((entry) => ({
          name: typeof entry.name === "string" ? entry.name : "",
          value: typeof entry.value === "number" ? entry.value : Number.NaN,
          unit:
            typeof entry.unit === "string" && entry.unit.trim().length > 0
              ? entry.unit
              : undefined
        }))
        .filter((entry) => entry.name.trim().length > 0 && Number.isFinite(entry.value));
      return { metrics };
    }

    if (raw && typeof raw === "object") {
      const payload = raw as Record<string, unknown>;
      if (Array.isArray(payload.metrics)) {
        return parseMetrics(payload.metrics);
      }
    }

    throw new Error("metrics command output must be { metrics: [...] } or an array");
  };

  adapter.traces = async (input) => {
    const command = tracesEnvName ? process.env[tracesEnvName] : undefined;
    if (command && command.trim().length > 0) {
      return parseTraces(await runJsonCommand(command, { cwd: projectDir }));
    }
    if (nativeTraces) {
      return nativeTraces(input);
    }
    return buildProviderTracesFromLocalArtifacts(projectDir, adapter.name, input);
  };

  adapter.metrics = async (input) => {
    const command = metricsEnvName ? process.env[metricsEnvName] : undefined;
    if (command && command.trim().length > 0) {
      return parseMetrics(await runJsonCommand(command, { cwd: projectDir }));
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
