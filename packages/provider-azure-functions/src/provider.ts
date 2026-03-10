import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import type {
  BuildArtifact,
  BuildPlan,
  BuildResult,
  DeployPlan,
  DeployResult,
  ProviderCredentialSchema,
  ProjectConfig,
  ProviderAdapter,
  ValidationResult
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

function missingRequiredCredentialErrors(schema: ProviderCredentialSchema): string[] {
  const errors: string[] = [];
  for (const field of schema.fields) {
    if (field.required === false) {
      continue;
    }
    const envValue = process.env[field.env];
    if (typeof envValue !== "string" || envValue.trim().length === 0) {
      errors.push(`missing credential env ${field.env} (${field.description})`);
    }
  }
  return errors;
}

export function createAzureFunctionsProvider(options: ProviderOptions): ProviderAdapter {
  const deployDir = resolve(options.projectDir, ".runfabric", "deploy", "azure-functions");

  return {
    name: "azure-functions",
    getCapabilities() {
      return azureFunctionsCapabilities;
    },
    getCredentialSchema() {
      return azureCredentialSchema;
    },
    async validate(project: ProjectConfig): Promise<ValidationResult> {
      const warnings: string[] = [];
      const errors: string[] = [];
      if (project.runtime !== "nodejs") {
        warnings.push("azure-functions runtime handling beyond nodejs is not implemented yet");
      }
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
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      await mkdir(deployDir, { recursive: true });
      const endpoint = `https://${project.service}.azurewebsites.net/api`;
      await writeFile(
        join(deployDir, "deployment.json"),
        JSON.stringify(
          {
            provider: "azure-functions",
            endpoint,
            executedSteps: plan.steps
          },
          null,
          2
        ),
        "utf8"
      );
      return { provider: "azure-functions", endpoint };
    }
  };
}
