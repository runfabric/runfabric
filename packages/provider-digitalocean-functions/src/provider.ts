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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function endpointFromDoResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const endpoint = readString(response.endpoint) || readString(response.url) || readString(response.webUrl);
  if (endpoint) {
    return endpoint;
  }

  if (isRecord(response.result)) {
    return readString(response.result.url) || readString(response.result.endpoint);
  }

  return undefined;
}

function doResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["name", "namespace", "region", "id"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultDigitalOceanDeployCommand(): string {
  return "doctl serverless deploy . --remote-build --output json";
}

function defaultDigitalOceanDestroyCommand(): string {
  return "doctl serverless undeploy . --output json";
}

export function createDigitalOceanFunctionsProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "digitalocean-functions",
    getCapabilities() {
      return digitalOceanFunctionsCapabilities;
    },
    getCredentialSchema() {
      return digitalOceanCredentialSchema;
    },
    async validate(_project: ProjectConfig): Promise<ValidationResult> {
      const errors = missingRequiredCredentialErrors(digitalOceanCredentialSchema);
      return { ok: errors.length === 0, warnings: [], errors };
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
        artifactPath: artifact.entry,
        artifactManifestPath: artifact.outputPath,
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      const extension = project.extensions?.["digitalocean-functions"];
      const region =
        typeof extension?.region === "string" ? extension.region : process.env.DIGITALOCEAN_REGION || "nyc1";
      const namespace =
        typeof extension?.namespace === "string"
          ? extension.namespace
          : process.env.DIGITALOCEAN_NAMESPACE || "default";
      const stage = project.stage || "default";
      const deploymentId = createDeploymentId("digitalocean-functions", project.service, stage);

      let endpoint = `https://faas-${region}.doserverless.co/api/v1/web/${namespace}/default/${project.service}`;
      let mode: "simulated" | "cli" = "simulated";
      let rawResponse: unknown;
      let resource: Record<string, unknown> | undefined;

      if (isRealDeployModeEnabled("RUNFABRIC_DIGITALOCEAN_REAL_DEPLOY")) {
        const deployCommand =
          process.env.RUNFABRIC_DIGITALOCEAN_DEPLOY_CMD || defaultDigitalOceanDeployCommand();
        const hasCommandOverride = Boolean(process.env.RUNFABRIC_DIGITALOCEAN_DEPLOY_CMD);

        rawResponse = await runJsonCommand(deployCommand, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: stage,
            RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
            RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath
          }
        });
        const parsedEndpoint = endpointFromDoResponse(rawResponse);
        if (parsedEndpoint) {
          endpoint = parsedEndpoint;
        } else if (hasCommandOverride) {
          throw new Error("digitalocean-functions deploy response does not include endpoint");
        }
        resource = {
          ...(doResourceMetadata(rawResponse) || {}),
          namespace,
          region,
          deployCommandSource: hasCommandOverride ? "override" : "builtin"
        };
        mode = "cli";
      }

      await writeDeploymentReceipt(options.projectDir, "digitalocean-functions", {
        provider: "digitalocean-functions",
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
        "digitalocean-functions",
        `deploy deploymentId=${deploymentId} mode=${mode} endpoint=${endpoint}`
      );

      return { provider: "digitalocean-functions", endpoint };
    },
    async invoke(input) {
      return invokeProviderViaDeployedEndpoint(options.projectDir, "digitalocean-functions", input);
    },
    async logs(input) {
      return buildProviderLogsFromLocalArtifacts(options.projectDir, "digitalocean-functions", input);
    },
    async destroy(project: ProjectConfig) {
      if (isRealDeployModeEnabled("RUNFABRIC_DIGITALOCEAN_REAL_DEPLOY")) {
        const destroyCommand =
          process.env.RUNFABRIC_DIGITALOCEAN_DESTROY_CMD || defaultDigitalOceanDestroyCommand();
        const result = await runShellCommand(destroyCommand, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: project.stage || "default"
          }
        });
        if (result.code !== 0) {
          throw new Error(result.stderr || result.stdout || "digitalocean-functions destroy command failed");
        }
      }

      await appendProviderLog(options.projectDir, "digitalocean-functions", "destroy local artifacts");
      await destroyProviderArtifacts(options.projectDir, "digitalocean-functions");
    }
  };
}
