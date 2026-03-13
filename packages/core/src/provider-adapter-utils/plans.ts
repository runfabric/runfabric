import type {
  BuildArtifact,
  BuildPlan,
  DeployPlan,
  ProviderAdapter
} from "../provider";
import type { ProjectConfig } from "../project";

export function createStandardProviderBuildPlan(
  provider: string,
  description: string | readonly string[]
): BuildPlan {
  return {
    provider,
    steps: Array.isArray(description) ? [...description] : [description]
  };
}

export function createStandardProviderDeployPlan(provider: string, artifact: BuildArtifact): DeployPlan {
  return {
    provider,
    artifactPath: artifact.entry,
    artifactManifestPath: artifact.outputPath,
    steps: [`deploy artifact from ${artifact.outputPath}`]
  };
}

export function createStandardProviderPlanOperations(
  provider: string,
  buildDescription: string | readonly string[]
): Pick<ProviderAdapter, "planBuild" | "planDeploy"> {
  return {
    planBuild: async (): Promise<BuildPlan> =>
      createStandardProviderBuildPlan(provider, buildDescription),
    planDeploy: async (_project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan> =>
      createStandardProviderDeployPlan(provider, artifact)
  };
}
