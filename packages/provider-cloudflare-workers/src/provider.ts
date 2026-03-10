import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import type {
  BuildArtifact,
  BuildPlan,
  BuildResult,
  DeployPlan,
  DeployResult,
  ProviderCredentialSchema,
  ProjectConfig,
  ProviderAdapter,
  ValidationResult
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

function missingRequiredCredentialErrors(schema: ProviderCredentialSchema): string[] {
  const errors: string[] = [];
  for (const field of schema.fields) {
    if (field.required === false) {
      continue;
    }
    const envValue = process.env[field.env];
    if (typeof envValue !== "string" || envValue.trim().length === 0) {
      errors.push(`missing credential env ${field.env} (${field.description})`);
    }
  }
  return errors;
}

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
  const mode = (process.env.RUNFABRIC_CLOUDFLARE_REAL_DEPLOY || "")
    .trim()
    .toLowerCase();
  return mode === "1" || mode === "true" || mode === "yes";
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
}

async function deployWorkerScript(params: {
  accountId: string;
  apiToken: string;
  scriptName: string;
  scriptContent: string;
}): Promise<void> {
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
}

export function createCloudflareWorkersProvider(options: ProviderOptions): ProviderAdapter {
  const deployDir = resolve(options.projectDir, ".runfabric", "deploy", "cloudflare-workers");

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
      if (project.runtime !== "nodejs") {
        warnings.push("cloudflare-workers runtime handling beyond nodejs is not implemented yet");
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
      await mkdir(deployDir, { recursive: true });

      const cloudflareExtension = project.extensions?.["cloudflare-workers"];
      const scriptName =
        typeof cloudflareExtension?.scriptName === "string"
          ? sanitizeScriptName(cloudflareExtension.scriptName)
          : sanitizeScriptName(project.service);

      const apiToken = process.env.CLOUDFLARE_API_TOKEN;
      const accountId = process.env.CLOUDFLARE_ACCOUNT_ID;
      const useRealDeploy = canUseRealDeploy();

      let endpoint = `https://${scriptName}.workers.dev`;
      let deploymentMode: "simulated" | "api" = "simulated";

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
        await deployWorkerScript({
          accountId,
          apiToken,
          scriptName,
          scriptContent
        });

        const subdomain = await resolveWorkersSubdomain(accountId, apiToken);
        if (subdomain) {
          endpoint = `https://${scriptName}.${subdomain}.workers.dev`;
        }
        deploymentMode = "api";
      }

      await writeFile(
        join(deployDir, "deployment.json"),
        JSON.stringify(
          {
            provider: "cloudflare-workers",
            endpoint,
            mode: deploymentMode,
            artifactPath: plan.artifactPath,
            artifactManifestPath: plan.artifactManifestPath,
            executedSteps: plan.steps
          },
          null,
          2
        ),
        "utf8"
      );
      return { provider: "cloudflare-workers", endpoint };
    }
  };
}
