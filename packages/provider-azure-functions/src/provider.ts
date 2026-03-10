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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function endpointFromAzureResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const candidates = [response.endpoint, response.url, response.defaultHostName];
  for (const candidate of candidates) {
    const endpoint = readString(candidate);
    if (endpoint) {
      if (endpoint.startsWith("http://") || endpoint.startsWith("https://")) {
        return endpoint;
      }
      return `https://${endpoint}`;
    }
  }

  if (isRecord(response.properties)) {
    const endpoint =
      readString(response.properties.defaultHostName) ||
      readString(response.properties.url) ||
      readString(response.properties.invokeUrlTemplate);
    if (endpoint) {
      if (endpoint.startsWith("http://") || endpoint.startsWith("https://")) {
        return endpoint;
      }
      return `https://${endpoint}`;
    }
  }

  return undefined;
}

function azureResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["id", "name", "state", "kind"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

export function createAzureFunctionsProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "azure-functions",
    getCapabilities() {
      return azureFunctionsCapabilities;
    },
    getCredentialSchema() {
      return azureCredentialSchema;
    },
    async validate(_project: ProjectConfig): Promise<ValidationResult> {
      const warnings: string[] = [];
      const errors: string[] = [];
      errors.push(...missingRequiredCredentialErrors(azureCredentialSchema));
      return { ok: errors.length === 0, warnings, errors };
    },
    async planBuild(): Promise<BuildPlan> {
      return {
        provider: "azure-functions",
        steps: ["prepare azure function metadata"]
      };
    },
    async build(): Promise<BuildResult> {
      return { artifacts: [] };
    },
    async planDeploy(_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> {
      return {
        provider: "azure-functions",
        artifactPath: artifact.entry,
        artifactManifestPath: artifact.outputPath,
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      const stage = project.stage || "default";
      const deploymentId = createDeploymentId("azure-functions", project.service, stage);

      let endpoint = `https://${project.service}.azurewebsites.net/api`;
      let mode: "simulated" | "cli" = "simulated";
      let rawResponse: unknown;
      let resource: Record<string, unknown> | undefined;

      if (isRealDeployModeEnabled("RUNFABRIC_AZURE_REAL_DEPLOY")) {
        const deployCommand = process.env.RUNFABRIC_AZURE_DEPLOY_CMD;
        if (!deployCommand) {
          throw new Error(
            "azure-functions real deploy mode requires RUNFABRIC_AZURE_DEPLOY_CMD returning JSON output"
          );
        }

        rawResponse = await runJsonCommand(deployCommand, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: stage,
            RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
            RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath
          }
        });
        const parsedEndpoint = endpointFromAzureResponse(rawResponse);
        if (!parsedEndpoint) {
          throw new Error("azure-functions deploy response does not include function app endpoint");
        }
        endpoint = parsedEndpoint;
        resource = azureResourceMetadata(rawResponse);
        mode = "cli";
      }

      await writeDeploymentReceipt(options.projectDir, "azure-functions", {
        provider: "azure-functions",
        service: project.service,
        stage,
        deploymentId,
        endpoint,
        mode,
        executedSteps: plan.steps,
        artifactPath: plan.artifactPath,
        artifactManifestPath: plan.artifactManifestPath,
        resource,
        rawResponse,
        createdAt: new Date().toISOString()
      });
      await appendProviderLog(
        options.projectDir,
        "azure-functions",
        `deploy deploymentId=${deploymentId} mode=${mode} endpoint=${endpoint}`
      );

      return { provider: "azure-functions", endpoint };
    },
    async invoke(input) {
      return invokeProviderViaDeployedEndpoint(options.projectDir, "azure-functions", input);
    },
    async logs(input) {
      return buildProviderLogsFromLocalArtifacts(options.projectDir, "azure-functions", input);
    },
    async destroy(project: ProjectConfig) {
      if (
        isRealDeployModeEnabled("RUNFABRIC_AZURE_REAL_DEPLOY") &&
        process.env.RUNFABRIC_AZURE_DESTROY_CMD
      ) {
        const result = await runShellCommand(process.env.RUNFABRIC_AZURE_DESTROY_CMD, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: project.stage || "default"
          }
        });
        if (result.code !== 0) {
          throw new Error(result.stderr || result.stdout || "azure-functions destroy command failed");
        }
      }

      await appendProviderLog(options.projectDir, "azure-functions", "destroy local artifacts");
      await destroyProviderArtifacts(options.projectDir, "azure-functions");
    }
  };
}
