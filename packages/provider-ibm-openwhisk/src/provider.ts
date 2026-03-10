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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function endpointFromIbmResponse(response: unknown): string | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const endpoint = readString(response.endpoint) || readString(response.url) || readString(response.apihost);
  if (endpoint) {
    return endpoint.startsWith("http") ? endpoint : `https://${endpoint}`;
  }

  if (isRecord(response.result)) {
    const nested = readString(response.result.url) || readString(response.result.endpoint);
    if (nested) {
      return nested.startsWith("http") ? nested : `https://${nested}`;
    }
  }

  return undefined;
}

function ibmResourceMetadata(response: unknown): Record<string, unknown> | undefined {
  if (!isRecord(response)) {
    return undefined;
  }

  const metadata: Record<string, unknown> = {};
  for (const key of ["namespace", "name", "version"]) {
    const value = readString(response[key]);
    if (value) {
      metadata[key] = value;
    }
  }
  return Object.keys(metadata).length > 0 ? metadata : undefined;
}

export function createIbmOpenWhiskProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "ibm-openwhisk",
    getCapabilities() {
      return ibmOpenWhiskCapabilities;
    },
    getCredentialSchema() {
      return ibmCredentialSchema;
    },
    async validate(_project: ProjectConfig): Promise<ValidationResult> {
      const errors = missingRequiredCredentialErrors(ibmCredentialSchema);
      return { ok: errors.length === 0, warnings: [], errors };
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
        artifactPath: artifact.entry,
        artifactManifestPath: artifact.outputPath,
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      const stage = project.stage || "default";
      const region = process.env.IBM_CLOUD_REGION || "us-south";
      const namespace = process.env.IBM_CLOUD_NAMESPACE || "default";
      const extension = project.extensions?.["ibm-openwhisk"];
      const effectiveNamespace =
        typeof extension?.namespace === "string" && extension.namespace.trim().length > 0
          ? extension.namespace
          : namespace;
      const deploymentId = createDeploymentId("ibm-openwhisk", project.service, stage);

      let endpoint = `https://${region}.functions.cloud.ibm.com/api/v1/web/${effectiveNamespace}/default/${project.service}`;
      let mode: "simulated" | "cli" = "simulated";
      let rawResponse: unknown;
      let resource: Record<string, unknown> | undefined;

      if (isRealDeployModeEnabled("RUNFABRIC_IBM_REAL_DEPLOY")) {
        const deployCommand = process.env.RUNFABRIC_IBM_DEPLOY_CMD;
        if (!deployCommand) {
          throw new Error(
            "ibm-openwhisk real deploy mode requires RUNFABRIC_IBM_DEPLOY_CMD returning JSON output"
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
        const parsedEndpoint = endpointFromIbmResponse(rawResponse);
        if (!parsedEndpoint) {
          throw new Error("ibm-openwhisk deploy response does not include endpoint");
        }
        endpoint = parsedEndpoint;
        resource = ibmResourceMetadata(rawResponse);
        mode = "cli";
      }

      await writeDeploymentReceipt(options.projectDir, "ibm-openwhisk", {
        provider: "ibm-openwhisk",
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
        "ibm-openwhisk",
        `deploy deploymentId=${deploymentId} mode=${mode} endpoint=${endpoint}`
      );

      return { provider: "ibm-openwhisk", endpoint };
    },
    async invoke(input) {
      return invokeProviderViaDeployedEndpoint(options.projectDir, "ibm-openwhisk", input);
    },
    async logs(input) {
      return buildProviderLogsFromLocalArtifacts(options.projectDir, "ibm-openwhisk", input);
    },
    async destroy(project: ProjectConfig) {
      if (isRealDeployModeEnabled("RUNFABRIC_IBM_REAL_DEPLOY") && process.env.RUNFABRIC_IBM_DESTROY_CMD) {
        const result = await runShellCommand(process.env.RUNFABRIC_IBM_DESTROY_CMD, {
          cwd: options.projectDir,
          env: {
            RUNFABRIC_SERVICE: project.service,
            RUNFABRIC_STAGE: project.stage || "default"
          }
        });
        if (result.code !== 0) {
          throw new Error(result.stderr || result.stdout || "ibm-openwhisk destroy command failed");
        }
      }

      await appendProviderLog(options.projectDir, "ibm-openwhisk", "destroy local artifacts");
      await destroyProviderArtifacts(options.projectDir, "ibm-openwhisk");
    }
  };
}
