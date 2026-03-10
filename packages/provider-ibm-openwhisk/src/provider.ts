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
import { ibmOpenWhiskCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const ibmCredentialSchema: ProviderCredentialSchema = {
  provider: "ibm-openwhisk",
  fields: [
    { env: "IBM_CLOUD_API_KEY", description: "IBM Cloud API key" },
    { env: "IBM_CLOUD_REGION", description: "IBM Cloud region" },
    { env: "IBM_CLOUD_NAMESPACE", description: "IBM Cloud Functions namespace" }
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

export function createIbmOpenWhiskProvider(options: ProviderOptions): ProviderAdapter {
  const deployDir = resolve(options.projectDir, ".runfabric", "deploy", "ibm-openwhisk");

  return {
    name: "ibm-openwhisk",
    getCapabilities() {
      return ibmOpenWhiskCapabilities;
    },
    getCredentialSchema() {
      return ibmCredentialSchema;
    },
    async validate(project: ProjectConfig): Promise<ValidationResult> {
      const warnings: string[] = [];
      const errors: string[] = [];
      if (project.runtime !== "nodejs") {
        warnings.push("ibm-openwhisk runtime handling beyond nodejs is not implemented yet");
      }
      errors.push(...missingRequiredCredentialErrors(ibmCredentialSchema));
      return { ok: errors.length === 0, warnings, errors };
    },
    async planBuild(): Promise<BuildPlan> {
      return {
        provider: "ibm-openwhisk",
        steps: ["prepare ibm openwhisk metadata"]
      };
    },
    async build(): Promise<BuildResult> {
      return { artifacts: [] };
    },
    async planDeploy(_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> {
      return {
        provider: "ibm-openwhisk",
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      await mkdir(deployDir, { recursive: true });
      const region = process.env.IBM_CLOUD_REGION || "us-south";
      const namespace = process.env.IBM_CLOUD_NAMESPACE || "default";
      const endpoint = `https://${region}.functions.cloud.ibm.com/api/v1/web/${namespace}/default/${project.service}`;
      await writeFile(
        join(deployDir, "deployment.json"),
        JSON.stringify(
          {
            provider: "ibm-openwhisk",
            endpoint,
            executedSteps: plan.steps
          },
          null,
          2
        ),
        "utf8"
      );
      return { provider: "ibm-openwhisk", endpoint };
    }
  };
}
