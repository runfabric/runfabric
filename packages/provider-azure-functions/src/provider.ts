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
import { azureFunctionsCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const azureCredentialSchema: ProviderCredentialSchema = {
  provider: "azure-functions",
  fields: [
    { env: "AZURE_TENANT_ID", description: "Azure Entra tenant ID" },
    { env: "AZURE_CLIENT_ID", description: "Azure service principal client ID" },
    { env: "AZURE_CLIENT_SECRET", description: "Azure service principal client secret" },
    { env: "AZURE_SUBSCRIPTION_ID", description: "Azure subscription ID" },
    { env: "AZURE_RESOURCE_GROUP", description: "Azure resource group name for function app" }
  ]
};

function normalizeHttpEndpoint(endpoint: string): string {
  if (endpoint.startsWith("http://") || endpoint.startsWith("https://")) {
    return endpoint;
  }
  return `https://${endpoint}`;
}

function endpointFromAzureResponse(response: unknown): string | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const direct =
    readNonEmptyString(response.endpoint) ||
    readNonEmptyString(response.url) ||
    readNonEmptyString(response.defaultHostName);
  if (direct) {
    return normalizeHttpEndpoint(direct);
  }

  if (!isRecordLike(response.properties)) {
    return undefined;
  }

  const nested =
    readNonEmptyString(response.properties.defaultHostName) ||
    readNonEmptyString(response.properties.url) ||
    readNonEmptyString(response.properties.invokeUrlTemplate);
  return nested ? normalizeHttpEndpoint(nested) : undefined;
}

function azureResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecordLike(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["id", "name", "state", "kind"]) {
    const value = readNonEmptyString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultAzureDeployCommand(functionAppName: string): string {
  return [
    `func azure functionapp publish ${JSON.stringify(functionAppName)} --no-build >/dev/null`,
    "&&",
    `az functionapp show --name ${JSON.stringify(functionAppName)} --resource-group "$AZURE_RESOURCE_GROUP" --output json`
  ].join(" ");
}

function defaultAzureDestroyCommand(functionAppName: string): string {
  return [
    "az functionapp delete",
    `--name ${JSON.stringify(functionAppName)}`,
    '--resource-group "$AZURE_RESOURCE_GROUP"',
    "--output none"
  ].join(" ");
}

function resolveFunctionAppName(project: ProjectConfig): string {
  const extension = project.extensions?.["azure-functions"];
  if (typeof extension?.functionAppName === "string" && extension.functionAppName.trim().length > 0) {
    return extension.functionAppName;
  }
  return process.env.AZURE_FUNCTION_APP_NAME || project.service;
}

async function deployAzureFunctions(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const functionAppName = resolveFunctionAppName(project);
  const deploymentId = createDeploymentId("azure-functions", project.service, stage);
  const deployState = await runStandardCliRealDeployIfEnabled({
    projectDir: options.projectDir,
    project,
    plan,
    stage,
    realDeployEnv: "RUNFABRIC_AZURE_REAL_DEPLOY",
    deployCommandEnv: "RUNFABRIC_AZURE_DEPLOY_CMD",
    defaultDeployCommand: defaultAzureDeployCommand(functionAppName),
    defaultEndpoint: `https://${functionAppName}.azurewebsites.net/api`,
    parseEndpoint: endpointFromAzureResponse,
    missingEndpointError: "azure-functions deploy response does not include function app endpoint",
    buildResource: (rawResponse) => azureResourceMetadata(rawResponse),
    extraResource: { functionAppName }
  });

  await writeDeploymentReceipt(options.projectDir, "azure-functions", {
    provider: "azure-functions",
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
    "azure-functions",
    `deploy deploymentId=${deploymentId} mode=${deployState.mode} endpoint=${deployState.endpoint}`
  );

  return { provider: "azure-functions", endpoint: deployState.endpoint };
}

async function destroyAzureFunctions(options: ProviderOptions, project: ProjectConfig): Promise<void> {
  if (isRealDeployModeEnabled("RUNFABRIC_AZURE_REAL_DEPLOY")) {
    const stage = project.stage || "default";
    const functionAppName = resolveFunctionAppName(project);
    const destroyCommand = process.env.RUNFABRIC_AZURE_DESTROY_CMD || defaultAzureDestroyCommand(functionAppName);
    const result = await runShellCommand(destroyCommand, {
      cwd: options.projectDir,
      env: {
        RUNFABRIC_SERVICE: project.service,
        RUNFABRIC_STAGE: stage
      }
    });
    if (result.code !== 0) {
      throw new Error(result.stderr || result.stdout || "azure-functions destroy command failed");
    }
  }

  await appendProviderLog(options.projectDir, "azure-functions", "destroy local artifacts");
  await destroyProviderArtifacts(options.projectDir, "azure-functions");
}

function validateAzureProvider(): ValidationResult {
  const errors: string[] = [];
  errors.push(...missingRequiredCredentialErrors(azureCredentialSchema));
  return { ok: errors.length === 0, warnings: [], errors };
}

const azurePlanOperations = createStandardProviderPlanOperations(
  "azure-functions",
  "prepare azure function metadata"
);

export function createAzureFunctionsProvider(options: ProviderOptions): ProviderAdapter {
  const observabilityOperations = createProviderNativeObservabilityOperations({
    projectDir: options.projectDir,
    provider: "azure-functions",
    realDeployEnv: "RUNFABRIC_AZURE_REAL_DEPLOY",
    tracesCommandEnv: "RUNFABRIC_AZURE_TRACES_CMD",
    metricsCommandEnv: "RUNFABRIC_AZURE_METRICS_CMD"
  });

  return {
    name: "azure-functions",
    getCapabilities: () => azureFunctionsCapabilities,
    getCredentialSchema: () => azureCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateAzureProvider(),
    planBuild: azurePlanOperations.planBuild,
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: azurePlanOperations.planDeploy,
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployAzureFunctions(options, project, plan),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "azure-functions", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "azure-functions", input),
    traces: observabilityOperations.traces,
    metrics: observabilityOperations.metrics,
    destroy: async (project: ProjectConfig) => destroyAzureFunctions(options, project)
  };
}
