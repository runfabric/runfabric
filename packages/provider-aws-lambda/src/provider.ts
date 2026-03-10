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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function endpointFromAwsResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const directCandidates = [
    response.endpoint,
    response.url,
    response.functionUrl,
    response.FunctionUrl
  ];

  for (const candidate of directCandidates) {
    const endpoint = readString(candidate);
    if (endpoint) {
      return endpoint;
    }
  }

  if (isRecord(response.FunctionUrlConfig)) {
    return readString(response.FunctionUrlConfig.FunctionUrl);
  }

  return undefined;
}

function awsResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["FunctionArn", "RevisionId", "Version", "Runtime"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

export function createAwsLambdaProvider(options: AwsProviderOptions): ProviderAdapter {
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
    async planBuild(): Promise<BuildPlan> {
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
        artifactPath: artifact.entry,
        artifactManifestPath: artifact.outputPath,
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      const stageExtension = project.extensions?.["aws-lambda"];
      const stage =
        typeof stageExtension?.stage === "string" && stageExtension.stage.trim().length > 0
          ? stageExtension.stage
          : project.stage || "default";
      const region =
        (typeof stageExtension?.region === "string" && stageExtension.region.trim().length > 0
          ? stageExtension.region
          : process.env.AWS_REGION) || "us-east-1";

      const deploymentId = createDeploymentId("aws-lambda", project.service, stage);
      let endpoint = `https://${project.service}.execute-api.${region}.amazonaws.com/${stage}`;
      let mode: "simulated" | "cli" = "simulated";
      let rawResponse: unknown;
      let resource: Record<string, unknown> | undefined;

      if (isRealDeployModeEnabled("RUNFABRIC_AWS_REAL_DEPLOY")) {
        const deployCommand = process.env.RUNFABRIC_AWS_DEPLOY_CMD;
        if (!deployCommand) {
          throw new Error(
            "aws-lambda real deploy mode requires RUNFABRIC_AWS_DEPLOY_CMD returning JSON output"
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
        const parsedEndpoint = endpointFromAwsResponse(rawResponse);
        if (!parsedEndpoint) {
          throw new Error("aws-lambda deploy response does not include an endpoint/function URL");
        }
        endpoint = parsedEndpoint;
        resource = awsResourceMetadata(rawResponse);
        mode = "cli";
      }

      await writeDeploymentReceipt(options.projectDir, "aws-lambda", {
        provider: "aws-lambda",
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
        "aws-lambda",
        `deploy deploymentId=${deploymentId} mode=${mode} endpoint=${endpoint}`
      );

      return {
        provider: "aws-lambda",
        endpoint
      };
    },
    async invoke(input) {
      return invokeProviderViaDeployedEndpoint(options.projectDir, "aws-lambda", input);
    },
    async logs(input) {
      return buildProviderLogsFromLocalArtifacts(options.projectDir, "aws-lambda", input);
    },
    async destroy(project: ProjectConfig) {
      const stage = project.stage || "default";
      if (isRealDeployModeEnabled("RUNFABRIC_AWS_REAL_DEPLOY") && process.env.RUNFABRIC_AWS_DESTROY_CMD) {
        const result = await runShellCommand(process.env.RUNFABRIC_AWS_DESTROY_CMD, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: stage
          }
        });
        if (result.code !== 0) {
          throw new Error(result.stderr || result.stdout || "aws-lambda destroy command failed");
        }
      }

      await appendProviderLog(options.projectDir, "aws-lambda", "destroy local artifacts");
      await destroyProviderArtifacts(options.projectDir, "aws-lambda");
    }
  };
}
