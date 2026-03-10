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

export function createNetlifyProvider(options: ProviderOptions): ProviderAdapter {
  const deployDir = resolve(options.projectDir, ".runfabric", "deploy", "netlify");

  return {
    name: "netlify",
    getCapabilities() {
      return netlifyCapabilities;
    },
    getCredentialSchema() {
      return netlifyCredentialSchema;
    },
    async validate(): Promise<ValidationResult> {
      const errors: string[] = [];
      errors.push(...missingRequiredCredentialErrors(netlifyCredentialSchema));
      return { ok: errors.length === 0, warnings: [], errors };
    },
    async planBuild(): Promise<BuildPlan> {
      return {
        provider: "netlify",
        steps: ["prepare netlify function metadata"]
      };
    },
    async build(): Promise<BuildResult> {
      return { artifacts: [] };
    },
    async planDeploy(_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> {
      return {
        provider: "netlify",
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      await mkdir(deployDir, { recursive: true });
      const endpoint = `https://${project.service}.netlify.app`;
      await writeFile(
        join(deployDir, "deployment.json"),
        JSON.stringify(
          {
            provider: "netlify",
            endpoint,
            executedSteps: plan.steps
          },
          null,
          2
        ),
        "utf8"
      );
      return { provider: "netlify", endpoint };
    }
  };
}
