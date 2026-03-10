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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function endpointFromVercelResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const endpoint =
    readString(response.endpoint) ||
    readString(response.url) ||
    readString(response.alias) ||
    readString(response.inspectorUrl);
  if (endpoint) {
    if (endpoint.startsWith("http://") || endpoint.startsWith("https://")) {
      return endpoint;
    }
    return `https://${endpoint}`;
  }

  if (Array.isArray(response.alias) && response.alias.length > 0) {
    const alias = readString(response.alias[0]);
    if (alias) {
      if (alias.startsWith("http://") || alias.startsWith("https://")) {
        return alias;
      }
      return `https://${alias}`;
    }
  }

  return undefined;
}

function vercelResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["id", "name", "projectId", "readyState"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

export function createVercelProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "vercel",
    getCapabilities() {
      return vercelCapabilities;
    },
    getCredentialSchema() {
      return vercelCredentialSchema;
    },
    async validate(): Promise<ValidationResult> {
      const errors = missingRequiredCredentialErrors(vercelCredentialSchema);
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
        artifactPath: artifact.entry,
        artifactManifestPath: artifact.outputPath,
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      const stage = project.stage || "default";
      const vercelExtension = project.extensions?.vercel;
      const projectName =
        typeof vercelExtension?.projectName === "string" && vercelExtension.projectName.trim().length > 0
          ? vercelExtension.projectName
          : project.service;
      const deploymentId = createDeploymentId("vercel", projectName, stage);

      let endpoint = `https://${projectName}.vercel.app`;
      let mode: "simulated" | "cli" = "simulated";
      let rawResponse: unknown;
      let resource: Record<string, unknown> | undefined;

      if (isRealDeployModeEnabled("RUNFABRIC_VERCEL_REAL_DEPLOY")) {
        const deployCommand = process.env.RUNFABRIC_VERCEL_DEPLOY_CMD;
        if (!deployCommand) {
          throw new Error(
            "vercel real deploy mode requires RUNFABRIC_VERCEL_DEPLOY_CMD returning JSON output"
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
        const parsedEndpoint = endpointFromVercelResponse(rawResponse);
        if (!parsedEndpoint) {
          throw new Error("vercel deploy response does not include deployment URL");
        }
        endpoint = parsedEndpoint;
        resource = vercelResourceMetadata(rawResponse);
        mode = "cli";
      }

      await writeDeploymentReceipt(options.projectDir, "vercel", {
        provider: "vercel",
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
        "vercel",
        `deploy deploymentId=${deploymentId} mode=${mode} endpoint=${endpoint}`
      );

      return { provider: "vercel", endpoint };
    },
    async invoke(input) {
      return invokeProviderViaDeployedEndpoint(options.projectDir, "vercel", input);
    },
    async logs(input) {
      return buildProviderLogsFromLocalArtifacts(options.projectDir, "vercel", input);
    },
    async destroy(project: ProjectConfig) {
      if (
        isRealDeployModeEnabled("RUNFABRIC_VERCEL_REAL_DEPLOY") &&
        process.env.RUNFABRIC_VERCEL_DESTROY_CMD
      ) {
        const result = await runShellCommand(process.env.RUNFABRIC_VERCEL_DESTROY_CMD, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: project.stage || "default"
          }
        });
        if (result.code !== 0) {
          throw new Error(result.stderr || result.stdout || "vercel destroy command failed");
        }
      }

      await appendProviderLog(options.projectDir, "vercel", "destroy local artifacts");
      await destroyProviderArtifacts(options.projectDir, "vercel");
    }
  };
}
