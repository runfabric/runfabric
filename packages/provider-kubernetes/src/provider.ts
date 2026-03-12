import type {
  BuildArtifact,
  BuildPlan,
  BuildResult,
  DeployPlan,
  ProjectConfig,
  ProviderAdapter,
  ProviderCredentialSchema,
  ValidationResult
} from "@runfabric/core";
import {
  buildProviderLogsFromLocalArtifacts,
  invokeProviderViaDeployedEndpoint,
  missingRequiredCredentialErrors
} from "@runfabric/core";
import { kubernetesCapabilities } from "./capabilities";
import { deployKubernetesProvider, destroyKubernetesProvider } from "./deploy";

const kubernetesCredentialSchema: ProviderCredentialSchema = {
  provider: "kubernetes",
  fields: [
    { env: "KUBECONFIG", description: "Path to kubeconfig file" },
    { env: "KUBE_CONTEXT", description: "Kubernetes context name", required: false },
    { env: "KUBE_NAMESPACE", description: "Kubernetes namespace", required: false }
  ]
};

const KUBERNETES_REAL_DEPLOY_ENV = "RUNFABRIC_KUBERNETES_REAL_DEPLOY";
const KUBERNETES_DEPLOY_CMD_ENV = "RUNFABRIC_KUBERNETES_DEPLOY_CMD";
const KUBERNETES_DESTROY_CMD_ENV = "RUNFABRIC_KUBERNETES_DESTROY_CMD";

function validateKubernetesProvider(): ValidationResult {
  const errors = missingRequiredCredentialErrors(kubernetesCredentialSchema);
  return { ok: errors.length === 0, warnings: [], errors };
}

function createBuildPlan(): BuildPlan {
  return {
    provider: "kubernetes",
    steps: ["prepare kubernetes deployment metadata"]
  };
}

function createDeployPlan(artifact: BuildArtifact): DeployPlan {
  return {
    provider: "kubernetes",
    artifactPath: artifact.entry,
    artifactManifestPath: artifact.outputPath,
    steps: [`deploy artifact from ${artifact.outputPath}`]
  };
}

export function createKubernetesProvider(options: { projectDir: string }): ProviderAdapter {
  return {
    name: "kubernetes",
    getCapabilities: () => kubernetesCapabilities,
    getCredentialSchema: () => kubernetesCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateKubernetesProvider(),
    planBuild: async (): Promise<BuildPlan> => createBuildPlan(),
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: async (_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> =>
      createDeployPlan(artifact),
    deploy: async (project: ProjectConfig, plan: DeployPlan) =>
      deployKubernetesProvider(options, project, plan, {
        realDeployEnv: KUBERNETES_REAL_DEPLOY_ENV,
        deployCommandEnv: KUBERNETES_DEPLOY_CMD_ENV
      }),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "kubernetes", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "kubernetes", input),
    destroy: async (project: ProjectConfig) =>
      destroyKubernetesProvider(options, project, {
        realDeployEnv: KUBERNETES_REAL_DEPLOY_ENV,
        destroyCommandEnv: KUBERNETES_DESTROY_CMD_ENV
      })
  };
}
