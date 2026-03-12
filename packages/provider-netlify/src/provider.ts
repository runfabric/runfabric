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
import { netlifyCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const netlifyCredentialSchema: ProviderCredentialSchema = {
  provider: "netlify",
  fields: [
    { env: "NETLIFY_AUTH_TOKEN", description: "Netlify personal access token" },
    { env: "NETLIFY_SITE_ID", description: "Netlify site ID" }
  ]
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function endpointFromNetlifyResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const endpoint = readString(response.endpoint) || readString(response.url) || readString(response.deploy_url);
  if (endpoint) {
    return endpoint;
  }

  if (isRecord(response.published_deploy)) {
    return readString(response.published_deploy.url);
  }

  return undefined;
}

function netlifyResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["id", "site_id", "state"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultNetlifyDeployCommand(): string {
  return 'netlify deploy --prod --json --dir "${RUNFABRIC_NETLIFY_PUBLISH_DIR:-.}"';
}

function defaultNetlifyDestroyCommand(): string {
  return 'netlify sites:delete --site "$NETLIFY_SITE_ID" --force';
}

function resolveSiteName(project: ProjectConfig): string {
  const extension = project.extensions?.netlify;
  if (typeof extension?.siteName === "string" && extension.siteName.trim().length > 0) {
    return extension.siteName;
  }
  return project.service;
}

async function runRealDeployIfEnabled(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan,
  stage: string,
  siteName: string
): Promise<{
  endpoint: string;
  mode: "simulated" | "cli";
  rawResponse?: unknown;
  resource?: Record<string, unknown>;
}> {
  const defaultEndpoint = `https://${siteName}.netlify.app`;
  if (!isRealDeployModeEnabled("RUNFABRIC_NETLIFY_REAL_DEPLOY")) {
    return { endpoint: defaultEndpoint, mode: "simulated" };
  }

  const deployCommand = process.env.RUNFABRIC_NETLIFY_DEPLOY_CMD || defaultNetlifyDeployCommand();
  const hasCommandOverride = Boolean(process.env.RUNFABRIC_NETLIFY_DEPLOY_CMD);
  const rawResponse = await runJsonCommand(deployCommand, {
    cwd: options.projectDir,
    env: {
      RUNFABRIC_SERVICE: project.service,
      RUNFABRIC_STAGE: stage,
      RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
      RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath
    }
  });

  const parsedEndpoint = endpointFromNetlifyResponse(rawResponse);
  if (!parsedEndpoint && hasCommandOverride) {
    throw new Error("netlify deploy response does not include deployment URL");
  }

  return {
    endpoint: parsedEndpoint || defaultEndpoint,
    mode: "cli",
    rawResponse,
    resource: {
      ...(netlifyResourceMetadata(rawResponse) || {}),
      siteName,
      deployCommandSource: hasCommandOverride ? "override" : "builtin"
    }
  };
}

async function deployNetlify(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const siteName = resolveSiteName(project);
  const deploymentId = createDeploymentId("netlify", siteName, stage);
  const deployState = await runRealDeployIfEnabled(options, project, plan, stage, siteName);

  await writeDeploymentReceipt(options.projectDir, "netlify", {
    provider: "netlify",
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
    "netlify",
    `deploy deploymentId=${deploymentId} mode=${deployState.mode} endpoint=${deployState.endpoint}`
  );

  return { provider: "netlify", endpoint: deployState.endpoint };
}

async function destroyNetlify(options: ProviderOptions, project: ProjectConfig): Promise<void> {
  if (isRealDeployModeEnabled("RUNFABRIC_NETLIFY_REAL_DEPLOY")) {
    const stage = project.stage || "default";
    const destroyCommand = process.env.RUNFABRIC_NETLIFY_DESTROY_CMD || defaultNetlifyDestroyCommand();
    const result = await runShellCommand(destroyCommand, {
      cwd: options.projectDir,
      env: {
        RUNFABRIC_SERVICE: project.service,
        RUNFABRIC_STAGE: stage
      }
    });
    if (result.code !== 0) {
      throw new Error(result.stderr || result.stdout || "netlify destroy command failed");
    }
  }

  await appendProviderLog(options.projectDir, "netlify", "destroy local artifacts");
  await destroyProviderArtifacts(options.projectDir, "netlify");
}

function validateNetlifyProvider(): ValidationResult {
  const errors = missingRequiredCredentialErrors(netlifyCredentialSchema);
  return { ok: errors.length === 0, warnings: [], errors };
}

function createBuildPlan(): BuildPlan {
  return {
    provider: "netlify",
    steps: ["prepare netlify function metadata"]
  };
}

function createDeployPlan(artifact: BuildArtifact): DeployPlan {
  return {
    provider: "netlify",
    artifactPath: artifact.entry,
    artifactManifestPath: artifact.outputPath,
    steps: [`deploy artifact from ${artifact.outputPath}`]
  };
}

export function createNetlifyProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "netlify",
    getCapabilities: () => netlifyCapabilities,
    getCredentialSchema: () => netlifyCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateNetlifyProvider(),
    planBuild: async (): Promise<BuildPlan> => createBuildPlan(),
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: async (_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> =>
      createDeployPlan(artifact),
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployNetlify(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "netlify", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "netlify", input),
    destroy: async (project: ProjectConfig) => destroyNetlify(options, project)
  };
}
