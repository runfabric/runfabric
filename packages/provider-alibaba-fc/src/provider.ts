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
import { alibabaFcCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const alibabaCredentialSchema: ProviderCredentialSchema = {
  provider: "alibaba-fc",
  fields: [
    { env: "ALICLOUD_ACCESS_KEY_ID", description: "Alibaba Cloud access key ID" },
    { env: "ALICLOUD_ACCESS_KEY_SECRET", description: "Alibaba Cloud access key secret" },
    { env: "ALICLOUD_REGION", description: "Alibaba Cloud region for Function Compute" }
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

export function createAlibabaFcProvider(options: ProviderOptions): ProviderAdapter {
  const deployDir = resolve(options.projectDir, ".runfabric", "deploy", "alibaba-fc");

  return {
    name: "alibaba-fc",
    getCapabilities() {
      return alibabaFcCapabilities;
    },
    getCredentialSchema() {
      return alibabaCredentialSchema;
    },
    async validate(project: ProjectConfig): Promise<ValidationResult> {
      const warnings: string[] = [];
      const errors: string[] = [];
      if (project.runtime !== "nodejs") {
        warnings.push("alibaba-fc runtime handling beyond nodejs is not implemented yet");
      }
      errors.push(...missingRequiredCredentialErrors(alibabaCredentialSchema));
      return { ok: errors.length === 0, warnings, errors };
    },
    async planBuild(): Promise<BuildPlan> {
      return {
        provider: "alibaba-fc",
        steps: ["prepare alibaba fc metadata"]
      };
    },
    async build(): Promise<BuildResult> {
      return { artifacts: [] };
    },
    async planDeploy(_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> {
      return {
        provider: "alibaba-fc",
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      await mkdir(deployDir, { recursive: true });
      const fcExtension = project.extensions?.["alibaba-fc"];
      const region =
        typeof fcExtension?.region === "string"
          ? fcExtension.region
          : process.env.ALICLOUD_REGION || "cn-hangzhou";
      const endpoint = `https://${project.service}.${region}.fcapp.run`;
      await writeFile(
        join(deployDir, "deployment.json"),
        JSON.stringify(
          {
            provider: "alibaba-fc",
            endpoint,
            executedSteps: plan.steps
          },
          null,
          2
        ),
        "utf8"
      );
      return { provider: "alibaba-fc", endpoint };
    }
  };
}
