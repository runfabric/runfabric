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
import { vercelCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const vercelCredentialSchema: ProviderCredentialSchema = {
  provider: "vercel",
  fields: [
    { env: "VERCEL_TOKEN", description: "Vercel API token" },
    { env: "VERCEL_ORG_ID", description: "Vercel team or user ID" },
    { env: "VERCEL_PROJECT_ID", description: "Vercel project ID" }
  ]
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function toHttpEndpoint(endpoint: string): string {
  if (endpoint.startsWith("http://") || endpoint.startsWith("https://")) {
    return endpoint;
  }
  return `https://${endpoint}`;
}

function endpointFromVercelResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const direct =
    readString(response.endpoint) ||
    readString(response.url) ||
    readString(response.alias) ||
    readString(response.inspectorUrl);
  if (direct) {
    return toHttpEndpoint(direct);
  }

  if (Array.isArray(response.alias) && response.alias.length > 0) {
    const alias = readString(response.alias[0]);
    return alias ? toHttpEndpoint(alias) : undefined;
  }

  return undefined;
}

function vercelResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["id", "name", "projectId", "readyState"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultVercelDeployCommand(): string {
  return 'vercel deploy --yes --prod --json --cwd "${RUNFABRIC_VERCEL_SOURCE_DIR:-.}"';
}

function defaultVercelDestroyCommand(): string {
  return 'vercel remove "$RUNFABRIC_SERVICE" --yes';
}

function resolveProjectName(project: ProjectConfig): string {
  const extension = project.extensions?.vercel;
  if (typeof extension?.projectName === "string" && extension.projectName.trim().length > 0) {
    return extension.projectName;
  }
  return project.service;
}

async function runRealDeployIfEnabled(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan,
  stage: string,
  projectName: string
): Promise<{
  endpoint: string;
  mode: "simulated" | "cli";
  rawResponse?: unknown;
  resource?: Record<string, unknown>;
}> {
  const defaultEndpoint = `https://${projectName}.vercel.app`;
  if (!isRealDeployModeEnabled("RUNFABRIC_VERCEL_REAL_DEPLOY")) {
    return { endpoint: defaultEndpoint, mode: "simulated" };
  }

  const deployCommand = process.env.RUNFABRIC_VERCEL_DEPLOY_CMD || defaultVercelDeployCommand();
  const hasCommandOverride = Boolean(process.env.RUNFABRIC_VERCEL_DEPLOY_CMD);
  const rawResponse = await runJsonCommand(deployCommand, {
    cwd: options.projectDir,
    env: {
      RUNFABRIC_SERVICE: project.service,
      RUNFABRIC_STAGE: stage,
      RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
      RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath
    }
  });

  const parsedEndpoint = endpointFromVercelResponse(rawResponse);
  if (!parsedEndpoint && hasCommandOverride) {
    throw new Error("vercel deploy response does not include deployment URL");
  }

  return {
    endpoint: parsedEndpoint || defaultEndpoint,
    mode: "cli",
    rawResponse,
    resource: {
      ...(vercelResourceMetadata(rawResponse) || {}),
      projectName,
      deployCommandSource: hasCommandOverride ? "override" : "builtin"
    }
  };
}

async function deployVercel(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const projectName = resolveProjectName(project);
  const deploymentId = createDeploymentId("vercel", projectName, stage);
  const deployState = await runRealDeployIfEnabled(options, project, plan, stage, projectName);

  await writeDeploymentReceipt(options.projectDir, "vercel", {
    provider: "vercel",
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
    "vercel",
    `deploy deploymentId=${deploymentId} mode=${deployState.mode} endpoint=${deployState.endpoint}`
  );

  return { provider: "vercel", endpoint: deployState.endpoint };
}

async function destroyVercel(options: ProviderOptions, project: ProjectConfig): Promise<void> {
  if (isRealDeployModeEnabled("RUNFABRIC_VERCEL_REAL_DEPLOY")) {
    const stage = project.stage || "default";
    const destroyCommand = process.env.RUNFABRIC_VERCEL_DESTROY_CMD || defaultVercelDestroyCommand();
    const result = await runShellCommand(destroyCommand, {
      cwd: options.projectDir,
      env: {
        RUNFABRIC_SERVICE: project.service,
        RUNFABRIC_STAGE: stage
      }
    });
    if (result.code !== 0) {
      throw new Error(result.stderr || result.stdout || "vercel destroy command failed");
    }
  }

  await appendProviderLog(options.projectDir, "vercel", "destroy local artifacts");
  await destroyProviderArtifacts(options.projectDir, "vercel");
}

function validateVercelProvider(): ValidationResult {
  const errors = missingRequiredCredentialErrors(vercelCredentialSchema);
  return { ok: errors.length === 0, warnings: [], errors };
}

function createBuildPlan(): BuildPlan {
  return {
    provider: "vercel",
    steps: ["prepare vercel function metadata"]
  };
}

function createDeployPlan(artifact: BuildArtifact): DeployPlan {
  return {
    provider: "vercel",
    artifactPath: artifact.entry,
    artifactManifestPath: artifact.outputPath,
    steps: [`deploy artifact from ${artifact.outputPath}`]
  };
}

export function createVercelProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "vercel",
    getCapabilities: () => vercelCapabilities,
    getCredentialSchema: () => vercelCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateVercelProvider(),
    planBuild: async (): Promise<BuildPlan> => createBuildPlan(),
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: async (_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> =>
      createDeployPlan(artifact),
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployVercel(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "vercel", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "vercel", input),
    destroy: async (project: ProjectConfig) => destroyVercel(options, project)
  };
}
