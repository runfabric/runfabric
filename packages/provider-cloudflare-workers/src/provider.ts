import { readFile } from "node:fs/promises";
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
  runShellCommand,
  writeDeploymentReceipt
} from "@runfabric/core";
import { cloudflareWorkersCapabilities } from "./capabilities";

interface ProviderOptions {
  projectDir: string;
}

const cloudflareCredentialSchema: ProviderCredentialSchema = {
  provider: "cloudflare-workers",
  fields: [
    { env: "CLOUDFLARE_API_TOKEN", description: "Cloudflare API token with Workers permissions" },
    { env: "CLOUDFLARE_ACCOUNT_ID", description: "Cloudflare account ID" }
  ]
};

function sanitizeScriptName(value: string): string {
  const normalized = value
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "-")
    .replace(/--+/g, "-")
    .replace(/^-+/, "")
    .replace(/-+$/, "");
  return normalized || "runfabric-worker";
}

function canUseRealDeploy(): boolean {
  return isRealDeployModeEnabled("RUNFABRIC_CLOUDFLARE_REAL_DEPLOY");
}

interface CloudflareSubdomainResponse {
  success: boolean;
  errors?: Array<{ message?: string }>;
  result?: { subdomain?: string };
}

async function resolveWorkersSubdomain(accountId: string, apiToken: string): Promise<string | undefined> {
  const response = await fetch(`https://api.cloudflare.com/client/v4/accounts/${accountId}/workers/subdomain`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${apiToken}`
    }
  });

  const json = (await response.json()) as CloudflareSubdomainResponse;
  if (!response.ok || !json.success) {
    return undefined;
  }
  return json.result?.subdomain;
}

interface CloudflareDeployResponse {
  success: boolean;
  errors?: Array<{ message?: string }>;
  result?: {
    id?: string;
    etag?: string;
  };
}

interface CloudflareDeleteResponse {
  success: boolean;
  errors?: Array<{ message?: string }>;
}

async function deployWorkerScript(params: {
  accountId: string;
  apiToken: string;
  scriptName: string;
  scriptContent: string;
}): Promise<CloudflareDeployResponse> {
  const response = await fetch(
    `https://api.cloudflare.com/client/v4/accounts/${params.accountId}/workers/scripts/${params.scriptName}`,
    {
      method: "PUT",
      headers: {
        Authorization: `Bearer ${params.apiToken}`,
        "content-type": "application/javascript"
      },
      body: params.scriptContent
    }
  );

  const json = (await response.json()) as CloudflareDeployResponse;
  if (!response.ok || !json.success) {
    const reason = json.errors?.map((item) => item.message).filter(Boolean).join("; ");
    throw new Error(`cloudflare-workers deploy failed${reason ? `: ${reason}` : ""}`);
  }
  return json;
}

async function deleteWorkerScript(params: {
  accountId: string;
  apiToken: string;
  scriptName: string;
}): Promise<void> {
  const response = await fetch(
    `https://api.cloudflare.com/client/v4/accounts/${params.accountId}/workers/scripts/${params.scriptName}`,
    {
      method: "DELETE",
      headers: {
        Authorization: `Bearer ${params.apiToken}`
      }
    }
  );

  const json = (await response.json()) as CloudflareDeleteResponse;
  if (!response.ok || !json.success) {
    const reason = json.errors?.map((item) => item.message).filter(Boolean).join("; ");
    throw new Error(`cloudflare-workers destroy failed${reason ? `: ${reason}` : ""}`);
  }
}

export function createCloudflareWorkersProvider(options: ProviderOptions): ProviderAdapter {
  return {
    name: "cloudflare-workers",
    getCapabilities() {
      return cloudflareWorkersCapabilities;
    },
    getCredentialSchema() {
      return cloudflareCredentialSchema;
    },
    async validate(project: ProjectConfig): Promise<ValidationResult> {
      const warnings: string[] = [];
      const errors: string[] = [];
      const runtime = project.runtime.trim().toLowerCase();
      if (!["nodejs", "javascript", "typescript", "edge"].includes(runtime)) {
        warnings.push("cloudflare-workers works best with nodejs/javascript/typescript/edge runtimes");
      }
      errors.push(...missingRequiredCredentialErrors(cloudflareCredentialSchema));
      return { ok: errors.length === 0, warnings, errors };
    },
    async planBuild(): Promise<BuildPlan> {
      return {
        provider: "cloudflare-workers",
        steps: ["prepare worker bundle metadata"]
      };
    },
    async build(): Promise<BuildResult> {
      return {
        artifacts: []
      };
    },
    async planDeploy(_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> {
      return {
        provider: "cloudflare-workers",
        artifactPath: artifact.entry,
        artifactManifestPath: artifact.outputPath,
        steps: [`deploy artifact from ${artifact.outputPath}`]
      };
    },
    async deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> {
      const cloudflareExtension = project.extensions?.["cloudflare-workers"];
      const scriptName =
        typeof cloudflareExtension?.scriptName === "string"
          ? sanitizeScriptName(cloudflareExtension.scriptName)
          : sanitizeScriptName(project.service);
      const stage = project.stage || "default";
      const deploymentId = createDeploymentId("cloudflare-workers", scriptName, stage);

      const apiToken = process.env.CLOUDFLARE_API_TOKEN;
      const accountId = process.env.CLOUDFLARE_ACCOUNT_ID;
      const useRealDeploy = canUseRealDeploy();

      let endpoint = `https://${scriptName}.workers.dev`;
      let deploymentMode: "simulated" | "api" = "simulated";
      let rawResponse: unknown;
      let resource: Record<string, unknown> | undefined;

      if (useRealDeploy) {
        if (!apiToken || !accountId) {
          throw new Error(
            "cloudflare-workers real deploy requested but CLOUDFLARE_API_TOKEN/CLOUDFLARE_ACCOUNT_ID are missing"
          );
        }
        if (!plan.artifactPath) {
          throw new Error("cloudflare-workers real deploy requires artifactPath in deploy plan");
        }

        const scriptContent = await readFile(plan.artifactPath, "utf8");
        const deployResponse = await deployWorkerScript({
          accountId,
          apiToken,
          scriptName,
          scriptContent
        });

        const subdomain = await resolveWorkersSubdomain(accountId, apiToken);
        if (subdomain) {
          endpoint = `https://${scriptName}.${subdomain}.workers.dev`;
        }
        rawResponse = deployResponse;
        resource = {
          accountId,
          scriptName,
          etag: deployResponse.result?.etag
        };
        deploymentMode = "api";
      }

      await writeDeploymentReceipt(options.projectDir, "cloudflare-workers", {
        provider: "cloudflare-workers",
        service: project.service,
        stage,
        deploymentId,
        endpoint,
        mode: deploymentMode,
        artifactPath: plan.artifactPath,
        artifactManifestPath: plan.artifactManifestPath,
        executedSteps: plan.steps,
        resource,
        rawResponse,
        createdAt: new Date().toISOString()
      });
      await appendProviderLog(
        options.projectDir,
        "cloudflare-workers",
        `deploy deploymentId=${deploymentId} mode=${deploymentMode} endpoint=${endpoint}`
      );

      return { provider: "cloudflare-workers", endpoint };
    },
    async invoke(input) {
      return invokeProviderViaDeployedEndpoint(options.projectDir, "cloudflare-workers", input);
    },
    async logs(input) {
      return buildProviderLogsFromLocalArtifacts(options.projectDir, "cloudflare-workers", input);
    },
    async destroy(project: ProjectConfig) {
      const stage = project.stage || "default";
      const cloudflareExtension = project.extensions?.["cloudflare-workers"];
      const scriptName =
        typeof cloudflareExtension?.scriptName === "string"
          ? sanitizeScriptName(cloudflareExtension.scriptName)
          : sanitizeScriptName(project.service);
      if (isRealDeployModeEnabled("RUNFABRIC_CLOUDFLARE_REAL_DEPLOY")) {
        if (process.env.RUNFABRIC_CLOUDFLARE_DESTROY_CMD) {
          const result = await runShellCommand(process.env.RUNFABRIC_CLOUDFLARE_DESTROY_CMD, {
            cwd: options.projectDir,
            env: {
              RUNFABRIC_SERVICE: project.service,
              RUNFABRIC_STAGE: stage
            }
          });
          if (result.code !== 0) {
            throw new Error(result.stderr || result.stdout || "cloudflare-workers destroy command failed");
          }
        } else {
          const apiToken = process.env.CLOUDFLARE_API_TOKEN;
          const accountId = process.env.CLOUDFLARE_ACCOUNT_ID;
          if (!apiToken || !accountId) {
            throw new Error(
              "cloudflare-workers real destroy requested but CLOUDFLARE_API_TOKEN/CLOUDFLARE_ACCOUNT_ID are missing"
            );
          }
          await deleteWorkerScript({
            accountId,
            apiToken,
            scriptName
          });
        }
      }

      await appendProviderLog(options.projectDir, "cloudflare-workers", "destroy local artifacts");
      await destroyProviderArtifacts(options.projectDir, "cloudflare-workers");
    }
  };
}
