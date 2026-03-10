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

export function createFlyMachinesProvider(options: ProviderOptions): ProviderAdapter {
  const deployDir = resolve(options.projectDir, ".runfabric", "deploy", "fly-machines");

  return {
    name: "fly-machines",
    getCapabilities() {
      return flyMachinesCapabilities;
    },
    getCredentialSchema() {
      return flyCredentialSchema;
    },
    async validate(project: ProjectConfig): Promise<ValidationResult> {
      const warnings: string[] = [];
      const errors: string[] = [];
      if (project.runtime !== "nodejs") {
        warnings.push("fly-machines runtime handling beyond nodejs is not implemented yet");
      }
      errors.push(...missingRequiredCredentialErrors(flyCredentialSchema));
      return { ok: errors.length === 0, warnings, errors };
    },
    async planBuild(): Promise<BuildPlan> {
      return {
        provider: "fly-machines",
        steps: ["prepare fly machines metadata"]
      };
    },
    async build(): Promise<BuildResult> {
      return { artifacts: [] };
    },
    async planDeploy(_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> {
      return {
        provider: "fly-machines",
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      await mkdir(deployDir, { recursive: true });
      const endpoint = `https://${project.service}.fly.dev`;
      await writeFile(
        join(deployDir, "deployment.json"),
        JSON.stringify(
          {
            provider: "fly-machines",
            endpoint,
            executedSteps: plan.steps
          },
          null,
          2
        ),
        "utf8"
      );
      return { provider: "fly-machines", endpoint };
    }
  };
}
