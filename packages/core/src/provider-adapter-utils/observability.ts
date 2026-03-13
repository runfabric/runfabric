import type {
  MetricsInput,
  MetricsResult,
  ProviderAdapter,
  TracesInput,
  TracesResult
} from "../provider";
import {
  buildProviderMetricsFromLocalArtifacts,
  buildProviderTracesFromLocalArtifacts,
  isRealDeployModeEnabled,
  readDeploymentReceipt,
  runProviderMetricsCommand,
  runProviderTracesCommand
} from "../provider-ops";

async function observabilityCommandEnv(input: {
  projectDir: string;
  provider: string;
  commandEnv?: Record<string, string | undefined>;
}): Promise<Record<string, string | undefined>> {
  const receipt = await readDeploymentReceipt(input.projectDir, input.provider);
  return {
    RUNFABRIC_SERVICE: receipt?.service,
    RUNFABRIC_STAGE: receipt?.stage,
    ...(input.commandEnv || {})
  };
}

function envCommand(envName: string): string | undefined {
  const raw = process.env[envName];
  return typeof raw === "string" && raw.trim().length > 0 ? raw : undefined;
}

export function createProviderNativeObservabilityOperations(input: {
  projectDir: string;
  provider: string;
  realDeployEnv: string;
  tracesCommandEnv: string;
  metricsCommandEnv: string;
  commandEnv?: Record<string, string | undefined>;
}): Pick<ProviderAdapter, "traces" | "metrics"> {
  return {
    traces: async (request: TracesInput): Promise<TracesResult> => {
      if (isRealDeployModeEnabled(input.realDeployEnv)) {
        const command = envCommand(input.tracesCommandEnv);
        if (command) {
          return runProviderTracesCommand(command, input.provider, {
            cwd: input.projectDir,
            env: await observabilityCommandEnv(input)
          });
        }
      }
      return buildProviderTracesFromLocalArtifacts(input.projectDir, input.provider, request);
    },
    metrics: async (request: MetricsInput): Promise<MetricsResult> => {
      if (isRealDeployModeEnabled(input.realDeployEnv)) {
        const command = envCommand(input.metricsCommandEnv);
        if (command) {
          return runProviderMetricsCommand(command, {
            cwd: input.projectDir,
            env: await observabilityCommandEnv(input)
          });
        }
      }
      return buildProviderMetricsFromLocalArtifacts(input.projectDir, input.provider, request);
    }
  };
}
