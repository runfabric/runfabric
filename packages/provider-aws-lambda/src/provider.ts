import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import type {
  BuildArtifact,
  BuildPlan,
  BuildResult,
  DeployPlan,
  DeployResult,
  InvokeInput,
  InvokeResult,
  LogsInput,
  LogsResult,
  ProviderCredentialSchema,
  ProjectConfig,
  ProviderAdapter,
  ValidationResult
} from "@runfabric/core";
import { awsLambdaCapabilities } from "./capabilities";

interface AwsProviderOptions {
  projectDir: string;
}

const awsLambdaCredentialSchema: ProviderCredentialSchema = {
  provider: "aws-lambda",
  fields: [
    { env: "AWS_ACCESS_KEY_ID", description: "AWS IAM access key ID" },
    { env: "AWS_SECRET_ACCESS_KEY", description: "AWS IAM secret access key" },
    { env: "AWS_REGION", description: "AWS region for Lambda deployment" }
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

export function createAwsLambdaProvider(options: AwsProviderOptions): ProviderAdapter {
  const projectDir = resolve(options.projectDir);
  const deployDir = resolve(projectDir, ".runfabric", "deploy", "aws-lambda");

  return {
    name: "aws-lambda",
    getCapabilities() {
      return awsLambdaCapabilities;
    },
    getCredentialSchema() {
      return awsLambdaCredentialSchema;
    },
    async validate(project: ProjectConfig): Promise<ValidationResult> {
      const warnings: string[] = [];
      const errors: string[] = [];

      if (!project.providers.includes("aws-lambda")) {
        warnings.push("project does not target aws-lambda in providers");
      }

      if (!project.triggers.some((trigger) => trigger.type === "http")) {
        warnings.push("aws-lambda provider currently expects at least one http trigger");
      }

      errors.push(...missingRequiredCredentialErrors(awsLambdaCredentialSchema));

      return {
        ok: errors.length === 0,
        warnings,
        errors
      };
    },
    async planBuild(_project: ProjectConfig): Promise<BuildPlan> {
      return {
        provider: "aws-lambda",
        steps: ["validate config", "prepare aws-lambda artifact manifest"]
      };
    },
    async build(_project: ProjectConfig, plan: BuildPlan): Promise<BuildResult> {
      return {
        artifacts: plan.steps.map((step, index) => ({
          provider: "aws-lambda",
          entry: step,
          outputPath: `aws-lambda-step-${index + 1}`
        }))
      };
    },
    async planDeploy(_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> {
      return {
        provider: "aws-lambda",
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      await mkdir(deployDir, { recursive: true });
      const region = process.env.AWS_REGION || "us-east-1";
      const awsExtension = project.extensions?.["aws-lambda"];
      const stage =
        typeof awsExtension?.stage === "string"
          ? awsExtension.stage
          : "prod";
      const endpoint = `https://${project.service}.execute-api.${region}.amazonaws.com/${stage}`;
      await writeFile(
        join(deployDir, "deployment.json"),
        JSON.stringify(
          {
            provider: "aws-lambda",
            endpoint,
            executedSteps: plan.steps
          },
          null,
          2
        ),
        "utf8"
      );

      return {
        provider: "aws-lambda",
        endpoint
      };
    },
    async invoke(input: InvokeInput): Promise<InvokeResult> {
      return {
        statusCode: 200,
        body: JSON.stringify({
          provider: "aws-lambda",
          message: "invoke stub",
          input
        })
      };
    },
    async logs(_input: LogsInput): Promise<LogsResult> {
      try {
        const receipt = await readFile(join(deployDir, "deployment.json"), "utf8");
        return {
          lines: [`deployment receipt loaded`, receipt]
        };
      } catch {
        return {
          lines: ["no deployment receipt found for aws-lambda yet"]
        };
      }
    }
  };
}
