import type { DeployPlan, DeployResult, ProjectConfig } from "@runfabric/core";
import {
  appendProviderLog,
  createDeploymentId,
  destroyProviderArtifacts,
  isRealDeployModeEnabled,
  runStandardCliRealDeployIfEnabled,
  runShellCommand,
  writeDeploymentReceipt
} from "@runfabric/core";
import { endpointFromKubernetesResponse, kubernetesResourceMetadata } from "./response";
import { resolveKubernetesSettings } from "./settings";

const defaultKubernetesDeployCommand =
  'kubectl apply -f "$RUNFABRIC_ARTIFACT_MANIFEST_PATH" --namespace "${RUNFABRIC_KUBE_NAMESPACE:-default}" -o json';
const defaultKubernetesDestroyCommand =
  'kubectl delete deployment "$RUNFABRIC_K8S_DEPLOYMENT_NAME" service "$RUNFABRIC_K8S_SERVICE_NAME" --namespace "${RUNFABRIC_KUBE_NAMESPACE:-default}" --ignore-not-found';

function buildProviderEnv(
  project: ProjectConfig,
  plan: DeployPlan,
  settings: ReturnType<typeof resolveKubernetesSettings>
): Record<string, string | undefined> {
  return {
    RUNFABRIC_SERVICE: project.service,
    RUNFABRIC_STAGE: settings.stage,
    RUNFABRIC_ARTIFACT_PATH: plan.artifactPath,
    RUNFABRIC_ARTIFACT_MANIFEST_PATH: plan.artifactManifestPath,
    RUNFABRIC_KUBE_NAMESPACE: settings.namespace,
    RUNFABRIC_KUBE_CONTEXT: settings.context,
    RUNFABRIC_K8S_DEPLOYMENT_NAME: settings.deploymentName,
    RUNFABRIC_K8S_SERVICE_NAME: settings.serviceName
  };
}

function buildKubernetesDeployContext(
  settings: ReturnType<typeof resolveKubernetesSettings>
): {
  commandEnv: Record<string, string | undefined>;
  resource: Record<string, string | undefined>;
} {
  return {
    commandEnv: {
      RUNFABRIC_KUBE_NAMESPACE: settings.namespace,
      RUNFABRIC_KUBE_CONTEXT: settings.context,
      RUNFABRIC_K8S_DEPLOYMENT_NAME: settings.deploymentName,
      RUNFABRIC_K8S_SERVICE_NAME: settings.serviceName
    },
    resource: {
      namespace: settings.namespace,
      context: settings.context,
      deploymentName: settings.deploymentName,
      serviceName: settings.serviceName
    }
  };
}

export async function deployKubernetesProvider(
  options: { projectDir: string },
  project: ProjectConfig,
  plan: DeployPlan,
  envNames: { realDeployEnv: string; deployCommandEnv: string }
): Promise<DeployResult> {
  const settings = resolveKubernetesSettings(project);
  const deployContext = buildKubernetesDeployContext(settings);
  const deploymentId = createDeploymentId("kubernetes", settings.deploymentName, settings.stage);
  const deployState = await runStandardCliRealDeployIfEnabled({
    projectDir: options.projectDir,
    project,
    plan,
    stage: settings.stage,
    realDeployEnv: envNames.realDeployEnv,
    deployCommandEnv: envNames.deployCommandEnv,
    defaultDeployCommand: defaultKubernetesDeployCommand,
    defaultEndpoint: settings.defaultEndpoint,
    parseEndpoint: endpointFromKubernetesResponse,
    missingEndpointError: "kubernetes deploy response does not include endpoint",
    env: deployContext.commandEnv,
    buildResource: (rawResponse) => kubernetesResourceMetadata(rawResponse),
    extraResource: deployContext.resource
  });

  await writeDeploymentReceipt(options.projectDir, "kubernetes", {
    provider: "kubernetes",
    service: project.service,
    stage: settings.stage,
    deploymentId,
    endpoint: deployState.endpoint,
    mode: deployState.mode,
    executedSteps: plan.steps,
    artifactPath: plan.artifactPath,
    artifactManifestPath: plan.artifactManifestPath,
    resource: deployState.resource,
    rawResponse: deployState.rawResponse,
    createdAt: new Date().toISOString()
  });
  await appendProviderLog(
    options.projectDir,
    "kubernetes",
    `deploy deploymentId=${deploymentId} mode=${deployState.mode} endpoint=${deployState.endpoint}`
  );

  return { provider: "kubernetes", endpoint: deployState.endpoint };
}

export async function destroyKubernetesProvider(
  options: { projectDir: string },
  project: ProjectConfig,
  envNames: { realDeployEnv: string; destroyCommandEnv: string }
): Promise<void> {
  const settings = resolveKubernetesSettings(project);
  if (isRealDeployModeEnabled(envNames.realDeployEnv)) {
    const destroyCommand = process.env[envNames.destroyCommandEnv] || defaultKubernetesDestroyCommand;
    const result = await runShellCommand(destroyCommand, {
      cwd: options.projectDir,
      env: buildProviderEnv(project, { provider: "kubernetes", steps: [] }, settings)
    });
    if (result.code !== 0) {
      throw new Error(result.stderr || result.stdout || "kubernetes destroy command failed");
    }
  }

  await appendProviderLog(options.projectDir, "kubernetes", "destroy local artifacts");
  await destroyProviderArtifacts(options.projectDir, "kubernetes");
}
