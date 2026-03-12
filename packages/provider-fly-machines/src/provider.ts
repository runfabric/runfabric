import type {
  BuildArtifact,
  BuildPlan,
  BuildResult,
  DeployPlan,
  DeployResult,
  ProjectConfig,
  ProviderAdapter,
  ProviderCredentialSchema,
  ValidationResult
} from "@runfabric/core";
import {
  appendProviderLog,
  buildProviderLogsFromLocalArtifacts,
  createDeploymentId,
  destroyProviderArtifacts,
  invokeProviderViaDeployedEndpoint,
  isRealDeployModeEnabled,
  missingRequiredCredentialErrors,
  runJsonCommand,
  runShellCommand,
  writeDeploymentReceipt
} from "@runfabric/core";
import { flyMachinesCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const flyCredentialSchema: ProviderCredentialSchema = {
  provider: "fly-machines",
  fields: [
    { env: "FLY_API_TOKEN", description: "Fly.io API token" },
    { env: "FLY_APP_NAME", description: "Fly app name" }
  ]
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function toHttpEndpoint(value: string): string {
  if (value.startsWith("http://") || value.startsWith("https://")) {
    return value;
  }
  return `https://${value}`;
}

function endpointFromFlyResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const direct = readString(response.endpoint) || readString(response.url) || readString(response.hostname);
  if (direct) {
    return toHttpEndpoint(direct);
  }

  if (!isRecord(response.app)) {
    return undefined;
  }

  const host = readString(response.app.hostname) || readString(response.app.name);
  if (!host) {
    return undefined;
  }
  return host.includes(".") ? `https://${host}` : `https://${host}.fly.dev`;
}

function flyResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["id", "name", "region", "status"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultFlyDeployCommand(appName: string): string {
  return ["flyctl deploy", "--json", "--remote-only", `--app ${JSON.stringify(appName)}`].join(" ");
}

function defaultFlyDestroyCommand(appName: string): string {
  return `flyctl apps destroy ${JSON.stringify(appName)} --yes`;
}

function resolveAppName(project: ProjectConfig): string {
  const extension = project.extensions?.["fly-machines"];
  if (typeof extension?.appName === "string" && extension.appName.trim().length > 0) {
    return extension.appName;
  }
  return process.env.FLY_APP_NAME || project.service;
}

async function runRealDeployIfEnabled(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan,
  stage: string,
  appName: string
): Promise<{
  endpoint: string;
  mode: "simulated" | "cli";
  rawResponse?: unknown;
  resource?: Record<string, unknown>;
}> {
  const defaultEndpoint = `https://${appName}.fly.dev`;
  if (!isRealDeployModeEnabled("RUNFABRIC_FLY_REAL_DEPLOY")) {
    return { endpoint: defaultEndpoint, mode: "simulated" };
  }

  const deployCommand = process.env.RUNFABRIC_FLY_DEPLOY_CMD || defaultFlyDeployCommand(appName);
  const hasCommandOverride = Boolean(process.env.RUNFABRIC_FLY_DEPLOY_CMD);
  const rawResponse = await runJsonCommand(deployCommand, {
    cwd: options.projectDir,
    env: {
      RUNFABRIC_SERVICE: project.service,
      RUNFABRIC_STAGE: stage,
      RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
      RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath
    }
  });

  const parsedEndpoint = endpointFromFlyResponse(rawResponse);
  if (!parsedEndpoint && hasCommandOverride) {
    throw new Error("fly-machines deploy response does not include endpoint");
  }

  return {
    endpoint: parsedEndpoint || defaultEndpoint,
    mode: "cli",
    rawResponse,
    resource: {
      ...(flyResourceMetadata(rawResponse) || {}),
      appName,
      deployCommandSource: hasCommandOverride ? "override" : "builtin"
    }
  };
}

async function deployFlyMachines(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const appName = resolveAppName(project);
  const deploymentId = createDeploymentId("fly-machines", appName, stage);
  const deployState = await runRealDeployIfEnabled(options, project, plan, stage, appName);

  await writeDeploymentReceipt(options.projectDir, "fly-machines", {
    provider: "fly-machines",
    service: project.service,
    stage,
    deploymentId,
    endpoint: deployState.endpoint,
    mode: deployState.mode,
    executedSteps: plan.steps,
    artifactPath: plan.artifactPath,
    artifactManifestPath: plan.artifactManifestPath,
    resource: deployState.resource,
    rawResponse: deployState.rawResponse,
    createdAt: new Date().toISOString()
  });
  await appendProviderLog(
    options.projectDir,
    "fly-machines",
    `deploy deploymentId=${deploymentId} mode=${deployState.mode} endpoint=${deployState.endpoint}`
  );

  return { provider: "fly-machines", endpoint: deployState.endpoint };
}

async function destroyFlyMachines(options: ProviderOptions, project: ProjectConfig): Promise<void> {
  if (isRealDeployModeEnabled("RUNFABRIC_FLY_REAL_DEPLOY")) {
    const stage = project.stage || "default";
    const destroyCommand = process.env.RUNFABRIC_FLY_DESTROY_CMD || defaultFlyDestroyCommand(resolveAppName(project));
    const result = await runShellCommand(destroyCommand, {
      cwd: options.projectDir,
      env: {
        RUNFABRIC_SERVICE: project.service,
        RUNFABRIC_STAGE: stage
      }
    });
    if (result.code !== 0) {
      throw new Error(result.stderr || result.stdout || "fly-machines destroy command failed");
    }
  }

  await appendProviderLog(options.projectDir, "fly-machines", "destroy local artifacts");
  await destroyProviderArtifacts(options.projectDir, "fly-machines");
}

function validateFlyProvider(): ValidationResult {
  const errors = missingRequiredCredentialErrors(flyCredentialSchema);
  return { ok: errors.length === 0, warnings: [], errors };
}

function createBuildPlan(): BuildPlan {
  return {
    provider: "fly-machines",
    steps: ["prepare fly machines metadata"]
  };
}

function createDeployPlan(artifact: BuildArtifact): DeployPlan {
  return {
    provider: "fly-machines",
    artifactPath: artifact.entry,
    artifactManifestPath: artifact.outputPath,
    steps: [`deploy artifact from ${artifact.outputPath}`]
  };
}

export function createFlyMachinesProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "fly-machines",
    getCapabilities: () => flyMachinesCapabilities,
    getCredentialSchema: () => flyCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateFlyProvider(),
    planBuild: async (): Promise<BuildPlan> => createBuildPlan(),
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: async (_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> =>
      createDeployPlan(artifact),
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployFlyMachines(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "fly-machines", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "fly-machines", input),
    destroy: async (project: ProjectConfig) => destroyFlyMachines(options, project)
  };
}
