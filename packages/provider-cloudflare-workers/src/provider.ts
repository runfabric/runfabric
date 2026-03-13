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
  createProviderNativeObservabilityOperations,
  createStandardProviderPlanOperations,
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

function resolveScriptName(project: ProjectConfig): string {
  const extension = project.extensions?.["cloudflare-workers"];
  if (typeof extension?.scriptName === "string") {
    return sanitizeScriptName(extension.scriptName);
  }
  return sanitizeScriptName(project.service);
}

function readRequiredCloudflareCredentials(): { apiToken: string; accountId: string } {
  const apiToken = process.env.CLOUDFLARE_API_TOKEN;
  const accountId = process.env.CLOUDFLARE_ACCOUNT_ID;
  if (!apiToken || !accountId) {
    throw new Error(
      "cloudflare-workers real deploy requested but CLOUDFLARE_API_TOKEN/CLOUDFLARE_ACCOUNT_ID are missing"
    );
  }
  return { apiToken, accountId };
}

async function runRealDeployIfEnabled(
  project: ProjectConfig,
  plan: DeployPlan,
  scriptName: string
): Promise<{
  endpoint: string;
  mode: "simulated" | "api";
  rawResponse?: unknown;
  resource?: Record<string, unknown>;
}> {
  const initialEndpoint = `https://${scriptName}.workers.dev`;
  if (!isRealDeployModeEnabled("RUNFABRIC_CLOUDFLARE_REAL_DEPLOY")) {
    return { endpoint: initialEndpoint, mode: "simulated" };
  }

  if (!plan.artifactPath) {
    throw new Error("cloudflare-workers real deploy requires artifactPath in deploy plan");
  }

  const { apiToken, accountId } = readRequiredCloudflareCredentials();
  const scriptContent = await readFile(plan.artifactPath, "utf8");
  const deployResponse = await deployWorkerScript({
    accountId,
    apiToken,
    scriptName,
    scriptContent
  });

  const subdomain = await resolveWorkersSubdomain(accountId, apiToken);
  const endpoint = subdomain ? `https://${scriptName}.${subdomain}.workers.dev` : initialEndpoint;

  return {
    endpoint,
    mode: "api",
    rawResponse: deployResponse,
    resource: {
      accountId,
      scriptName,
      etag: deployResponse.result?.etag
    }
  };
}

async function deployCloudflareWorkers(
  options: ProviderOptions,
  project: ProjectConfig,
  plan: DeployPlan
): Promise<DeployResult> {
  const stage = project.stage || "default";
  const scriptName = resolveScriptName(project);
  const deploymentId = createDeploymentId("cloudflare-workers", scriptName, stage);
  const deployState = await runRealDeployIfEnabled(project, plan, scriptName);

  await writeDeploymentReceipt(options.projectDir, "cloudflare-workers", {
    provider: "cloudflare-workers",
    service: project.service,
    stage,
    deploymentId,
    endpoint: deployState.endpoint,
    mode: deployState.mode,
    artifactPath: plan.artifactPath,
    artifactManifestPath: plan.artifactManifestPath,
    executedSteps: plan.steps,
    resource: deployState.resource,
    rawResponse: deployState.rawResponse,
    createdAt: new Date().toISOString()
  });
  await appendProviderLog(
    options.projectDir,
    "cloudflare-workers",
    `deploy deploymentId=${deploymentId} mode=${deployState.mode} endpoint=${deployState.endpoint}`
  );

  return { provider: "cloudflare-workers", endpoint: deployState.endpoint };
}

async function runCloudflareDestroyCommand(
  options: ProviderOptions,
  project: ProjectConfig,
  stage: string
): Promise<void> {
  const command = process.env.RUNFABRIC_CLOUDFLARE_DESTROY_CMD;
  if (!command) {
    return;
  }

  const result = await runShellCommand(command, {
    cwd: options.projectDir,
    env: {
      RUNFABRIC_SERVICE: project.service,
      RUNFABRIC_STAGE: stage
    }
  });
  if (result.code !== 0) {
    throw new Error(result.stderr || result.stdout || "cloudflare-workers destroy command failed");
  }
}

async function destroyCloudflareWorkers(options: ProviderOptions, project: ProjectConfig): Promise<void> {
  if (isRealDeployModeEnabled("RUNFABRIC_CLOUDFLARE_REAL_DEPLOY")) {
    const stage = project.stage || "default";
    await runCloudflareDestroyCommand(options, project, stage);

    if (!process.env.RUNFABRIC_CLOUDFLARE_DESTROY_CMD) {
      const { apiToken, accountId } = readRequiredCloudflareCredentials();
      await deleteWorkerScript({ accountId, apiToken, scriptName: resolveScriptName(project) });
    }
  }

  await appendProviderLog(options.projectDir, "cloudflare-workers", "destroy local artifacts");
  await destroyProviderArtifacts(options.projectDir, "cloudflare-workers");
}

function validateCloudflareProvider(project: ProjectConfig): ValidationResult {
  const warnings: string[] = [];
  const errors: string[] = [];

  const runtime = project.runtime.trim().toLowerCase();
  if (!["nodejs", "javascript", "typescript", "edge"].includes(runtime)) {
    warnings.push("cloudflare-workers works best with nodejs/javascript/typescript/edge runtimes");
  }

  errors.push(...missingRequiredCredentialErrors(cloudflareCredentialSchema));
  return { ok: errors.length === 0, warnings, errors };
}

const cloudflarePlanOperations = createStandardProviderPlanOperations(
  "cloudflare-workers",
  "prepare worker bundle metadata"
);

export function createCloudflareWorkersProvider(options: ProviderOptions): ProviderAdapter {
  const observabilityOperations = createProviderNativeObservabilityOperations({
    projectDir: options.projectDir,
    provider: "cloudflare-workers",
    realDeployEnv: "RUNFABRIC_CLOUDFLARE_REAL_DEPLOY",
    tracesCommandEnv: "RUNFABRIC_CLOUDFLARE_TRACES_CMD",
    metricsCommandEnv: "RUNFABRIC_CLOUDFLARE_METRICS_CMD"
  });

  return {
    name: "cloudflare-workers",
    getCapabilities: () => cloudflareWorkersCapabilities,
    getCredentialSchema: () => cloudflareCredentialSchema,
    validate: async (project: ProjectConfig): Promise<ValidationResult> =>
      validateCloudflareProvider(project),
    planBuild: cloudflarePlanOperations.planBuild,
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: cloudflarePlanOperations.planDeploy,
    deploy: async (project: ProjectConfig, plan: DeployPlan): Promise<DeployResult> =>
      deployCloudflareWorkers(options, project, plan),
    invoke: async (input) =>
      invokeProviderViaDeployedEndpoint(options.projectDir, "cloudflare-workers", input),
    logs: async (input) =>
      buildProviderLogsFromLocalArtifacts(options.projectDir, "cloudflare-workers", input),
    traces: observabilityOperations.traces,
    metrics: observabilityOperations.metrics,
    destroy: async (project: ProjectConfig) => destroyCloudflareWorkers(options, project)
  };
}
