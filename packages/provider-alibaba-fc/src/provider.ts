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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function endpointFromAlibabaResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const endpoint = readString(response.endpoint) || readString(response.url) || readString(response.internetAddress);
  if (endpoint) {
    if (endpoint.startsWith("http://") || endpoint.startsWith("https://")) {
      return endpoint;
    }
    return `https://${endpoint}`;
  }

  if (isRecord(response.result)) {
    const nested = readString(response.result.url) || readString(response.result.endpoint);
    if (nested) {
      if (nested.startsWith("http://") || nested.startsWith("https://")) {
        return nested;
      }
      return `https://${nested}`;
    }
  }

  return undefined;
}

function alibabaResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["serviceName", "functionName", "region", "requestId"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultAlibabaDeployCommand(): string {
  return "s deploy --output json";
}

function defaultAlibabaDestroyCommand(): string {
  return "s remove --yes";
}

export function createAlibabaFcProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "alibaba-fc",
    getCapabilities() {
      return alibabaFcCapabilities;
    },
    getCredentialSchema() {
      return alibabaCredentialSchema;
    },
    async validate(_project: ProjectConfig): Promise<ValidationResult> {
      const errors = missingRequiredCredentialErrors(alibabaCredentialSchema);
      return { ok: errors.length === 0, warnings: [], errors };
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
        artifactPath: artifact.entry,
        artifactManifestPath: artifact.outputPath,
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      const fcExtension = project.extensions?.["alibaba-fc"];
      const region =
        typeof fcExtension?.region === "string" ? fcExtension.region : process.env.ALICLOUD_REGION || "cn-hangzhou";
      const stage = project.stage || "default";
      const deploymentId = createDeploymentId("alibaba-fc", project.service, stage);

      let endpoint = `https://${project.service}.${region}.fcapp.run`;
      let mode: "simulated" | "cli" = "simulated";
      let rawResponse: unknown;
      let resource: Record<string, unknown> | undefined;

      if (isRealDeployModeEnabled("RUNFABRIC_ALIBABA_REAL_DEPLOY")) {
        const deployCommand = process.env.RUNFABRIC_ALIBABA_DEPLOY_CMD || defaultAlibabaDeployCommand();
        const hasCommandOverride = Boolean(process.env.RUNFABRIC_ALIBABA_DEPLOY_CMD);

        rawResponse = await runJsonCommand(deployCommand, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: stage,
            RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
            RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath
          }
        });
        const parsedEndpoint = endpointFromAlibabaResponse(rawResponse);
        if (parsedEndpoint) {
          endpoint = parsedEndpoint;
        } else if (hasCommandOverride) {
          throw new Error("alibaba-fc deploy response does not include endpoint");
        }
        resource = {
          ...(alibabaResourceMetadata(rawResponse) || {}),
          region,
          deployCommandSource: hasCommandOverride ? "override" : "builtin"
        };
        mode = "cli";
      }

      await writeDeploymentReceipt(options.projectDir, "alibaba-fc", {
        provider: "alibaba-fc",
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
        "alibaba-fc",
        `deploy deploymentId=${deploymentId} mode=${mode} endpoint=${endpoint}`
      );

      return { provider: "alibaba-fc", endpoint };
    },
    async invoke(input) {
      return invokeProviderViaDeployedEndpoint(options.projectDir, "alibaba-fc", input);
    },
    async logs(input) {
      return buildProviderLogsFromLocalArtifacts(options.projectDir, "alibaba-fc", input);
    },
    async destroy(project: ProjectConfig) {
      if (isRealDeployModeEnabled("RUNFABRIC_ALIBABA_REAL_DEPLOY")) {
        const destroyCommand = process.env.RUNFABRIC_ALIBABA_DESTROY_CMD || defaultAlibabaDestroyCommand();
        const result = await runShellCommand(destroyCommand, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: project.stage || "default"
          }
        });
        if (result.code !== 0) {
          throw new Error(result.stderr || result.stdout || "alibaba-fc destroy command failed");
        }
      }

      await appendProviderLog(options.projectDir, "alibaba-fc", "destroy local artifacts");
      await destroyProviderArtifacts(options.projectDir, "alibaba-fc");
    }
  };
}
