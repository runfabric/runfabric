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
  createProviderNativeObservabilityOperations,
  createStandardProviderPlanOperations,
  createDeploymentId,
  destroyProviderArtifacts,
  invokeProviderViaDeployedEndpoint,
  isRecordLike,
  isRealDeployModeEnabled,
  missingRequiredCredentialErrors,
  readNonEmptyString,
  runStandardCliRealDeployIfEnabled,
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

function endpointFromNetlifyResponse(response: unknown): string | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const endpoint =
    readNonEmptyString(response.endpoint) ||
    readNonEmptyString(response.url) ||
    readNonEmptyString(response.deploy_url);
  if (endpoint) {
    return endpoint;
  }

  if (isRecordLike(response.published_deploy)) {
    return readNonEmptyString(response.published_deploy.url);
  }

  return undefined;
}

function netlifyResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["id", "site_id", "state"]) {
    const value = readNonEmptyString(response[key]);
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

async function deployNetlify(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const siteName = resolveSiteName(project);
  const deploymentId = createDeploymentId("netlify", siteName, stage);
  const deployState = await runStandardCliRealDeployIfEnabled({
    projectDir: options.projectDir,
    project,
    plan,
    stage,
    realDeployEnv: "RUNFABRIC_NETLIFY_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_NETLIFY_DEPLOY_CMD",
    defaultDeployCommand: defaultNetlifyDeployCommand(),
    defaultEndpoint: `https://${siteName}.netlify.app`,
    parseEndpoint: endpointFromNetlifyResponse,
    missingEndpointError: "netlify deploy response does not include deployment URL",
    buildResource: (rawResponse) => netlifyResourceMetadata(rawResponse),
    extraResource: { siteName }
  });

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

const netlifyPlanOperations = createStandardProviderPlanOperations(
  "netlify",
  "prepare netlify function metadata"
);

export function createNetlifyProvider(options: ProviderOptions): ProviderAdapter {
  const observabilityOperations = createProviderNativeObservabilityOperations({
    projectDir: options.projectDir,
    provider: "netlify",
    realDeployEnv: "RUNFABRIC_NETLIFY_REAL_DEPLOY",
    tracesCommandEnv: "RUNFABRIC_NETLIFY_TRACES_CMD",
    metricsCommandEnv: "RUNFABRIC_NETLIFY_METRICS_CMD"
  });

  return {
    name: "netlify",
    getCapabilities: () => netlifyCapabilities,
    getCredentialSchema: () => netlifyCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateNetlifyProvider(),
    planBuild: netlifyPlanOperations.planBuild,
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: netlifyPlanOperations.planDeploy,
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployNetlify(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "netlify", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "netlify", input),
    traces: observabilityOperations.traces,
    metrics: observabilityOperations.metrics,
    destroy: async (project: ProjectConfig) => destroyNetlify(options, project)
  };
}
