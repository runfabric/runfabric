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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function endpointFromNetlifyResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const endpoint = readString(response.endpoint) || readString(response.url) || readString(response.deploy_url);
  if (endpoint) {
    return endpoint;
  }

  if (isRecord(response.published_deploy)) {
    return readString(response.published_deploy.url);
  }

  return undefined;
}

function netlifyResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["id", "site_id", "state"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

export function createNetlifyProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "netlify",
    getCapabilities() {
      return netlifyCapabilities;
    },
    getCredentialSchema() {
      return netlifyCredentialSchema;
    },
    async validate(): Promise<ValidationResult> {
      const errors = missingRequiredCredentialErrors(netlifyCredentialSchema);
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
        artifactPath: artifact.entry,
        artifactManifestPath: artifact.outputPath,
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      const stage = project.stage || "default";
      const netlifyExtension = project.extensions?.netlify;
      const siteName =
        typeof netlifyExtension?.siteName === "string" && netlifyExtension.siteName.trim().length > 0
          ? netlifyExtension.siteName
          : project.service;
      const deploymentId = createDeploymentId("netlify", siteName, stage);

      let endpoint = `https://${siteName}.netlify.app`;
      let mode: "simulated" | "cli" = "simulated";
      let rawResponse: unknown;
      let resource: Record<string, unknown> | undefined;

      if (isRealDeployModeEnabled("RUNFABRIC_NETLIFY_REAL_DEPLOY")) {
        const deployCommand = process.env.RUNFABRIC_NETLIFY_DEPLOY_CMD;
        if (!deployCommand) {
          throw new Error(
            "netlify real deploy mode requires RUNFABRIC_NETLIFY_DEPLOY_CMD returning JSON output"
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
        const parsedEndpoint = endpointFromNetlifyResponse(rawResponse);
        if (!parsedEndpoint) {
          throw new Error("netlify deploy response does not include deployment URL");
        }
        endpoint = parsedEndpoint;
        resource = netlifyResourceMetadata(rawResponse);
        mode = "cli";
      }

      await writeDeploymentReceipt(options.projectDir, "netlify", {
        provider: "netlify",
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
        "netlify",
        `deploy deploymentId=${deploymentId} mode=${mode} endpoint=${endpoint}`
      );

      return { provider: "netlify", endpoint };
    },
    async invoke(input) {
      return invokeProviderViaDeployedEndpoint(options.projectDir, "netlify", input);
    },
    async logs(input) {
      return buildProviderLogsFromLocalArtifacts(options.projectDir, "netlify", input);
    },
    async destroy(project: ProjectConfig) {
      if (
        isRealDeployModeEnabled("RUNFABRIC_NETLIFY_REAL_DEPLOY") &&
        process.env.RUNFABRIC_NETLIFY_DESTROY_CMD
      ) {
        const result = await runShellCommand(process.env.RUNFABRIC_NETLIFY_DESTROY_CMD, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: project.stage || "default"
          }
        });
        if (result.code !== 0) {
          throw new Error(result.stderr || result.stdout || "netlify destroy command failed");
        }
      }

      await appendProviderLog(options.projectDir, "netlify", "destroy local artifacts");
      await destroyProviderArtifacts(options.projectDir, "netlify");
    }
  };
}
