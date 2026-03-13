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
  createProviderNativeObservabilityOperations,
  createStandardProviderPlanOperations,
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

const kubernetesPlanOperations = createStandardProviderPlanOperations(
  "kubernetes",
  "prepare kubernetes deployment metadata"
);

export function createKubernetesProvider(options: { projectDir: string }): ProviderAdapter {
  const observabilityOperations = createProviderNativeObservabilityOperations({
    projectDir: options.projectDir,
    provider: "kubernetes",
    realDeployEnv: KUBERNETES_REAL_DEPLOY_ENV,
    tracesCommandEnv: "RUNFABRIC_KUBERNETES_TRACES_CMD",
    metricsCommandEnv: "RUNFABRIC_KUBERNETES_METRICS_CMD"
  });

  return {
    name: "kubernetes",
    getCapabilities: () => kubernetesCapabilities,
    getCredentialSchema: () => kubernetesCredentialSchema,
    validate: async (): Promise<ValidationResult> => validateKubernetesProvider(),
    planBuild: kubernetesPlanOperations.planBuild,
    build: async (): Promise<BuildResult> => ({ artifacts: [] }),
    planDeploy: kubernetesPlanOperations.planDeploy,
    deploy: async (project: ProjectConfig, plan: DeployPlan) =>
      deployKubernetesProvider(options, project, plan, {
        realDeployEnv: KUBERNETES_REAL_DEPLOY_ENV,
        deployCommandEnv: KUBERNETES_DEPLOY_CMD_ENV
      }),
    invoke: async (input) => invokeProviderViaDeployedEndpoint(options.projectDir, "kubernetes", input),
    logs: async (input) => buildProviderLogsFromLocalArtifacts(options.projectDir, "kubernetes", input),
    traces: observabilityOperations.traces,
    metrics: observabilityOperations.metrics,
    destroy: async (project: ProjectConfig) =>
      destroyKubernetesProvider(options, project, {
        realDeployEnv: KUBERNETES_REAL_DEPLOY_ENV,
        destroyCommandEnv: KUBERNETES_DESTROY_CMD_ENV
      })
  };
}
