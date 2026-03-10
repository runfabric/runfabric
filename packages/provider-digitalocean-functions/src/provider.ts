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
import { digitalOceanFunctionsCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const digitalOceanCredentialSchema: ProviderCredentialSchema = {
  provider: "digitalocean-functions",
  fields: [
    { env: "DIGITALOCEAN_ACCESS_TOKEN", description: "DigitalOcean API token" },
    { env: "DIGITALOCEAN_NAMESPACE", description: "DigitalOcean Functions namespace" }
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

export function createDigitalOceanFunctionsProvider(options: ProviderOptions): ProviderAdapter {
  const deployDir = resolve(options.projectDir, ".runfabric", "deploy", "digitalocean-functions");

  return {
    name: "digitalocean-functions",
    getCapabilities() {
      return digitalOceanFunctionsCapabilities;
    },
    getCredentialSchema() {
      return digitalOceanCredentialSchema;
    },
    async validate(project: ProjectConfig): Promise<ValidationResult> {
      const warnings: string[] = [];
      const errors: string[] = [];
      if (project.runtime !== "nodejs") {
        warnings.push("digitalocean-functions runtime handling beyond nodejs is not implemented yet");
      }
      errors.push(...missingRequiredCredentialErrors(digitalOceanCredentialSchema));
      return { ok: errors.length === 0, warnings, errors };
    },
    async planBuild(): Promise<BuildPlan> {
      return {
        provider: "digitalocean-functions",
        steps: ["prepare digitalocean functions metadata"]
      };
    },
    async build(): Promise<BuildResult> {
      return { artifacts: [] };
    },
    async planDeploy(_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> {
      return {
        provider: "digitalocean-functions",
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      await mkdir(deployDir, { recursive: true });
      const region = process.env.DIGITALOCEAN_REGION || "nyc1";
      const namespace = process.env.DIGITALOCEAN_NAMESPACE || "default";
      const endpoint = `https://faas-${region}.doserverless.co/api/v1/web/${namespace}/default/${project.service}`;
      await writeFile(
        join(deployDir, "deployment.json"),
        JSON.stringify(
          {
            provider: "digitalocean-functions",
            endpoint,
            executedSteps: plan.steps
          },
          null,
          2
        ),
        "utf8"
      );
      return { provider: "digitalocean-functions", endpoint };
    }
  };
}
