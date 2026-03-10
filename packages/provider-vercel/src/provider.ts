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

export function createVercelProvider(options: ProviderOptions): ProviderAdapter {
  const deployDir = resolve(options.projectDir, ".runfabric", "deploy", "vercel");

  return {
    name: "vercel",
    getCapabilities() {
      return vercelCapabilities;
    },
    getCredentialSchema() {
      return vercelCredentialSchema;
    },
    async validate(): Promise<ValidationResult> {
      const errors: string[] = [];
      errors.push(...missingRequiredCredentialErrors(vercelCredentialSchema));
      return { ok: errors.length === 0, warnings: [], errors };
    },
    async planBuild(): Promise<BuildPlan> {
      return {
        provider: "vercel",
        steps: ["prepare vercel function metadata"]
      };
    },
    async build(): Promise<BuildResult> {
      return { artifacts: [] };
    },
    async planDeploy(_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> {
      return {
        provider: "vercel",
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      await mkdir(deployDir, { recursive: true });
      const endpoint = `https://${project.service}.vercel.app`;
      await writeFile(
        join(deployDir, "deployment.json"),
        JSON.stringify(
          {
            provider: "vercel",
            endpoint,
            executedSteps: plan.steps
          },
          null,
          2
        ),
        "utf8"
      );
      return { provider: "vercel", endpoint };
    }
  };
}
