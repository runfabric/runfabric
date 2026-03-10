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
import { gcpFunctionsCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const gcpCredentialSchema: ProviderCredentialSchema = {
  provider: "gcp-functions",
  fields: [
    { env: "GCP_PROJECT_ID", description: "Google Cloud project ID" },
    {
      env: "GCP_SERVICE_ACCOUNT_KEY",
      description: "Google Cloud service account JSON key (raw JSON or base64-decoded content)"
    }
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

export function createGcpFunctionsProvider(options: ProviderOptions): ProviderAdapter {
  const deployDir = resolve(options.projectDir, ".runfabric", "deploy", "gcp-functions");

  return {
    name: "gcp-functions",
    getCapabilities() {
      return gcpFunctionsCapabilities;
    },
    getCredentialSchema() {
      return gcpCredentialSchema;
    },
    async validate(project: ProjectConfig): Promise<ValidationResult> {
      const warnings: string[] = [];
      const errors: string[] = [];
      if (project.runtime !== "nodejs") {
        warnings.push("gcp-functions runtime handling beyond nodejs is not implemented yet");
      }
      errors.push(...missingRequiredCredentialErrors(gcpCredentialSchema));
      return { ok: errors.length === 0, warnings, errors };
    },
    async planBuild(): Promise<BuildPlan> {
      return {
        provider: "gcp-functions",
        steps: ["prepare gcp function metadata"]
      };
    },
    async build(): Promise<BuildResult> {
      return { artifacts: [] };
    },
    async planDeploy(_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> {
      return {
        provider: "gcp-functions",
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      await mkdir(deployDir, { recursive: true });
      const gcpExtension = project.extensions?.["gcp-functions"];
      const projectId = process.env.GCP_PROJECT_ID || "project-id";
      const region =
        typeof gcpExtension?.region === "string"
          ? gcpExtension.region
          : process.env.GCP_REGION || "us-central1";
      const endpoint = `https://${region}-${projectId}.cloudfunctions.net/${project.service}`;
      await writeFile(
        join(deployDir, "deployment.json"),
        JSON.stringify(
          {
            provider: "gcp-functions",
            endpoint,
            executedSteps: plan.steps
          },
          null,
          2
        ),
        "utf8"
      );
      return { provider: "gcp-functions", endpoint };
    }
  };
}
