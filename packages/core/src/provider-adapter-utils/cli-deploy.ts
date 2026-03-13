import type { DeployPlan } from "../provider";
import type { ProjectConfig } from "../project";
import { isRealDeployModeEnabled, runJsonCommand } from "../provider-ops";

function deployCommandEnv(input: {
  project: ProjectConfig;
  plan: DeployPlan;
  stage: string;
  env?: Record<string, string | undefined>;
}): Record<string, string | undefined> {
  return {
    RUNFABRIC_SERVICE: input.project.service,
    RUNFABRIC_STAGE: input.stage,
    RUNFABRIC_ARTIFACT_PATH: input.plan.artifactPath,
    RUNFABRIC_ARTIFACT_MANIFEST_PATH: input.plan.artifactManifestPath,
    ...(input.env || {})
  };
}

export async function runCliDeployIfEnabled(input: {
  projectDir: string;
  project: ProjectConfig;
  plan: DeployPlan;
  stage: string;
  realDeployEnv: string;
  deployCommandEnv: string;
  defaultDeployCommand: string;
  defaultEndpoint: string;
  parseEndpoint(response: unknown): string | undefined;
  missingEndpointError: string;
  env?: Record<string, string | undefined>;
  buildResource?: (
    response: unknown,
    context: { hasCommandOverride: boolean }
  ) => Record<string, unknown> | undefined;
}): Promise<{
  endpoint: string;
  mode: "simulated" | "cli";
  rawResponse?: unknown;
  resource?: Record<string, unknown>;
}> {
  if (!isRealDeployModeEnabled(input.realDeployEnv)) {
    return { endpoint: input.defaultEndpoint, mode: "simulated" };
  }

  const deployCommand = process.env[input.deployCommandEnv] || input.defaultDeployCommand;
  const hasCommandOverride = Boolean(process.env[input.deployCommandEnv]);
  const rawResponse = await runJsonCommand(deployCommand, {
    cwd: input.projectDir,
    env: deployCommandEnv(input)
  });

  const parsedEndpoint = input.parseEndpoint(rawResponse);
  if (!parsedEndpoint && hasCommandOverride) {
    throw new Error(input.missingEndpointError);
  }

  return {
    endpoint: parsedEndpoint || input.defaultEndpoint,
    mode: "cli",
    rawResponse,
    resource: input.buildResource?.(rawResponse, { hasCommandOverride })
  };
}

export async function runStandardCliRealDeployIfEnabled(input: {
  projectDir: string;
  project: ProjectConfig;
  plan: DeployPlan;
  stage: string;
  realDeployEnv: string;
  deployCommandEnv: string;
  defaultDeployCommand: string;
  defaultEndpoint: string;
  parseEndpoint(response: unknown): string | undefined;
  missingEndpointError: string;
  env?: Record<string, string | undefined>;
  extraResource?: Record<string, unknown>;
  buildResource?: (
    response: unknown,
    context: { hasCommandOverride: boolean }
  ) => Record<string, unknown> | undefined;
}): Promise<{
  endpoint: string;
  mode: "simulated" | "cli";
  rawResponse?: unknown;
  resource?: Record<string, unknown>;
}> {
  return runCliDeployIfEnabled({
    ...input,
    buildResource: (rawResponse, context) => ({
      ...(input.buildResource?.(rawResponse, context) || {}),
      ...(input.extraResource || {}),
      deployCommandSource: context.hasCommandOverride ? "override" : "builtin"
    })
  });
}
