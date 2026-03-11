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
import { flyMachinesCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const flyCredentialSchema: ProviderCredentialSchema = {
  provider: "fly-machines",
  fields: [
    { env: "FLY_API_TOKEN", description: "Fly.io API token" },
    { env: "FLY_APP_NAME", description: "Fly app name" }
  ]
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function endpointFromFlyResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const endpoint = readString(response.endpoint) || readString(response.url) || readString(response.hostname);
  if (endpoint) {
    if (endpoint.startsWith("http://") || endpoint.startsWith("https://")) {
      return endpoint;
    }
    return `https://${endpoint}`;
  }

  if (isRecord(response.app)) {
    const host = readString(response.app.hostname) || readString(response.app.name);
    if (host) {
      if (host.includes(".")) {
        return `https://${host}`;
      }
      return `https://${host}.fly.dev`;
    }
  }

  return undefined;
}

function flyResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["id", "name", "region", "status"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultFlyDeployCommand(appName: string): string {
  return [
    "flyctl deploy",
    "--json",
    "--remote-only",
    `--app ${JSON.stringify(appName)}`
  ].join(" ");
}

function defaultFlyDestroyCommand(appName: string): string {
  return `flyctl apps destroy ${JSON.stringify(appName)} --yes`;
}

export function createFlyMachinesProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "fly-machines",
    getCapabilities() {
      return flyMachinesCapabilities;
    },
    getCredentialSchema() {
      return flyCredentialSchema;
    },
    async validate(project: ProjectConfig): Promise<ValidationResult> {
      const errors: string[] = [];
      errors.push(...missingRequiredCredentialErrors(flyCredentialSchema));
      return { ok: errors.length === 0, warnings: [], errors };
    },
    async planBuild(): Promise<BuildPlan> {
      return {
        provider: "fly-machines",
        steps: ["prepare fly machines metadata"]
      };
    },
    async build(): Promise<BuildResult> {
      return { artifacts: [] };
    },
    async planDeploy(_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> {
      return {
        provider: "fly-machines",
        artifactPath: artifact.entry,
        artifactManifestPath: artifact.outputPath,
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      const extension = project.extensions?.["fly-machines"];
      const appName =
        typeof extension?.appName === "string" && extension.appName.trim().length > 0
          ? extension.appName
          : process.env.FLY_APP_NAME || project.service;
      const stage = project.stage || "default";
      const deploymentId = createDeploymentId("fly-machines", appName, stage);

      let endpoint = `https://${appName}.fly.dev`;
      let mode: "simulated" | "cli" = "simulated";
      let rawResponse: unknown;
      let resource: Record<string, unknown> | undefined;

      if (isRealDeployModeEnabled("RUNFABRIC_FLY_REAL_DEPLOY")) {
        const deployCommand = process.env.RUNFABRIC_FLY_DEPLOY_CMD || defaultFlyDeployCommand(appName);
        const hasCommandOverride = Boolean(process.env.RUNFABRIC_FLY_DEPLOY_CMD);

        rawResponse = await runJsonCommand(deployCommand, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: stage,
            RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
            RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath
          }
        });
        const parsedEndpoint = endpointFromFlyResponse(rawResponse);
        if (parsedEndpoint) {
          endpoint = parsedEndpoint;
        } else if (hasCommandOverride) {
          throw new Error("fly-machines deploy response does not include endpoint");
        }
        resource = {
          ...(flyResourceMetadata(rawResponse) || {}),
          appName,
          deployCommandSource: hasCommandOverride ? "override" : "builtin"
        };
        mode = "cli";
      }

      await writeDeploymentReceipt(options.projectDir, "fly-machines", {
        provider: "fly-machines",
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
        "fly-machines",
        `deploy deploymentId=${deploymentId} mode=${mode} endpoint=${endpoint}`
      );

      return { provider: "fly-machines", endpoint };
    },
    async invoke(input) {
      return invokeProviderViaDeployedEndpoint(options.projectDir, "fly-machines", input);
    },
    async logs(input) {
      return buildProviderLogsFromLocalArtifacts(options.projectDir, "fly-machines", input);
    },
    async destroy(project: ProjectConfig) {
      const extension = project.extensions?.["fly-machines"];
      const appName =
        typeof extension?.appName === "string" && extension.appName.trim().length > 0
          ? extension.appName
          : process.env.FLY_APP_NAME || project.service;
      if (isRealDeployModeEnabled("RUNFABRIC_FLY_REAL_DEPLOY")) {
        const destroyCommand = process.env.RUNFABRIC_FLY_DESTROY_CMD || defaultFlyDestroyCommand(appName);
        const result = await runShellCommand(destroyCommand, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: project.stage || "default"
          }
        });
        if (result.code !== 0) {
          throw new Error(result.stderr || result.stdout || "fly-machines destroy command failed");
        }
      }

      await appendProviderLog(options.projectDir, "fly-machines", "destroy local artifacts");
      await destroyProviderArtifacts(options.projectDir, "fly-machines");
    }
  };
}
