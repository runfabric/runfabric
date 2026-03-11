import type { ProviderAdapter } from "@runfabric/core";
import {
  buildProviderMetricsFromLocalArtifacts,
  buildProviderTracesFromLocalArtifacts,
  runJsonCommand,
  type MetricsResult,
  type TraceRecord,
  type TracesResult
} from "@runfabric/core";
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

export function createProviderRegistry(projectDir: string): Record<string, ProviderAdapter> {
  return {
    "aws-lambda": withObservability(createAwsLambdaProvider({ projectDir }), projectDir),
    "gcp-functions": withObservability(createGcpFunctionsProvider({ projectDir }), projectDir),
    "azure-functions": withObservability(createAzureFunctionsProvider({ projectDir }), projectDir),
    "cloudflare-workers": withObservability(createCloudflareWorkersProvider({ projectDir }), projectDir),
    vercel: withObservability(createVercelProvider({ projectDir }), projectDir),
    netlify: withObservability(createNetlifyProvider({ projectDir }), projectDir),
    "alibaba-fc": withObservability(createAlibabaFcProvider({ projectDir }), projectDir),
    "digitalocean-functions": withObservability(
      createDigitalOceanFunctionsProvider({ projectDir }),
      projectDir
    ),
    "fly-machines": withObservability(createFlyMachinesProvider({ projectDir }), projectDir),
    "ibm-openwhisk": withObservability(createIbmOpenWhiskProvider({ projectDir }), projectDir)
  };
}
