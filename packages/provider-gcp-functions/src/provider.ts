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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function endpointFromGcpResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const directCandidates = [response.endpoint, response.url, response.uri];
  for (const candidate of directCandidates) {
    const endpoint = readString(candidate);
    if (endpoint) {
      return endpoint;
    }
  }

  if (isRecord(response.httpsTrigger)) {
    const endpoint = readString(response.httpsTrigger.url);
    if (endpoint) {
      return endpoint;
    }
  }

  if (isRecord(response.serviceConfig)) {
    const endpoint = readString(response.serviceConfig.uri);
    if (endpoint) {
      return endpoint;
    }
  }

  return undefined;
}

function gcpResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["name", "state", "updateTime", "buildId"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function defaultGcpDeployCommand(region: string): string {
  return [
    'gcloud functions deploy "$RUNFABRIC_SERVICE"',
    "--gen2",
    `--region=${JSON.stringify(region)}`,
    '--runtime="${RUNFABRIC_GCP_RUNTIME:-nodejs20}"',
    '--entry-point="${RUNFABRIC_GCP_ENTRY_POINT:-handler}"',
    "--trigger-http",
    "--allow-unauthenticated",
    '--source="${RUNFABRIC_GCP_SOURCE_DIR:-.}"',
    "--format=json"
  ].join(" ");
}

function defaultGcpDestroyCommand(region: string): string {
  return [
    'gcloud functions delete "$RUNFABRIC_SERVICE"',
    "--gen2",
    `--region=${JSON.stringify(region)}`,
    "--quiet",
    "--format=json"
  ].join(" ");
}

export function createGcpFunctionsProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "gcp-functions",
    getCapabilities() {
      return gcpFunctionsCapabilities;
    },
    getCredentialSchema() {
      return gcpCredentialSchema;
    },
    async validate(_project: ProjectConfig): Promise<ValidationResult> {
      const errors = missingRequiredCredentialErrors(gcpCredentialSchema);
      return { ok: errors.length === 0, warnings: [], errors };
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
        artifactPath: artifact.entry,
        artifactManifestPath: artifact.outputPath,
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      const gcpExtension = project.extensions?.["gcp-functions"];
      const projectId = process.env.GCP_PROJECT_ID || "project-id";
      const region =
        typeof gcpExtension?.region === "string" ? gcpExtension.region : process.env.GCP_REGION || "us-central1";
      const stage = project.stage || "default";
      const deploymentId = createDeploymentId("gcp-functions", project.service, stage);

      let endpoint = `https://${region}-${projectId}.cloudfunctions.net/${project.service}`;
      let mode: "simulated" | "cli" = "simulated";
      let rawResponse: unknown;
      let resource: Record<string, unknown> | undefined;

      if (isRealDeployModeEnabled("RUNFABRIC_GCP_REAL_DEPLOY")) {
        const deployCommand = process.env.RUNFABRIC_GCP_DEPLOY_CMD || defaultGcpDeployCommand(region);
        const hasCommandOverride = Boolean(process.env.RUNFABRIC_GCP_DEPLOY_CMD);

        rawResponse = await runJsonCommand(deployCommand, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: stage,
            RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
            RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath
          }
        });
        const parsedEndpoint = endpointFromGcpResponse(rawResponse);
        if (parsedEndpoint) {
          endpoint = parsedEndpoint;
        } else if (hasCommandOverride) {
          throw new Error("gcp-functions deploy response does not include endpoint URL");
        }
        resource = {
          ...(gcpResourceMetadata(rawResponse) || {}),
          deployCommandSource: hasCommandOverride ? "override" : "builtin"
        };
        mode = "cli";
      }

      await writeDeploymentReceipt(options.projectDir, "gcp-functions", {
        provider: "gcp-functions",
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
        "gcp-functions",
        `deploy deploymentId=${deploymentId} mode=${mode} endpoint=${endpoint}`
      );

      return { provider: "gcp-functions", endpoint };
    },
    async invoke(input) {
      return invokeProviderViaDeployedEndpoint(options.projectDir, "gcp-functions", input);
    },
    async logs(input) {
      return buildProviderLogsFromLocalArtifacts(options.projectDir, "gcp-functions", input);
    },
    async destroy(project: ProjectConfig) {
      const gcpExtension = project.extensions?.["gcp-functions"];
      const region =
        typeof gcpExtension?.region === "string" ? gcpExtension.region : process.env.GCP_REGION || "us-central1";
      if (isRealDeployModeEnabled("RUNFABRIC_GCP_REAL_DEPLOY")) {
        const destroyCommand = process.env.RUNFABRIC_GCP_DESTROY_CMD || defaultGcpDestroyCommand(region);
        const result = await runShellCommand(destroyCommand, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: project.stage || "default"
          }
        });
        if (result.code !== 0) {
          throw new Error(result.stderr || result.stdout || "gcp-functions destroy command failed");
        }
      }

      await appendProviderLog(options.projectDir, "gcp-functions", "destroy local artifacts");
      await destroyProviderArtifacts(options.projectDir, "gcp-functions");
    }
  };
}
