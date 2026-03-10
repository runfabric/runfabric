import type { ProviderCapabilities } from "./capabilities";
import type { ProviderCredentialSchema } from "./credentials";
import type { ProjectConfig } from "./project";

export interface ValidationResult {
  ok: boolean;
  warnings?: string[];
  errors?: string[];
}

export interface BuildPlan {
  provider: string;
  steps: string[];
}

export interface BuildArtifact {
  provider: string;
  entry: string;
  outputPath: string;
}

export interface BuildResult {
  artifacts: BuildArtifact[];
}

export interface DeployPlan {
  provider: string;
  steps: string[];
  artifactPath?: string;
  artifactManifestPath?: string;
}

export interface DeployResult {
  provider: string;
  endpoint?: string;
}

export interface InvokeInput {
  provider: string;
  payload?: string;
}

export interface InvokeResult {
  statusCode: number;
  body?: string;
}

export interface LogsInput {
  provider: string;
  since?: string;
}

export interface LogsResult {
  lines: string[];
}

export interface ProviderAdapter {
  name: string;
  getCapabilities(): ProviderCapabilities;
  getCredentialSchema?(): ProviderCredentialSchema;
  validate(project: ProjectConfig): Promise<ValidationResult>;
  planBuild(project: ProjectConfig): Promise<BuildPlan>;
  build(project: ProjectConfig, plan: BuildPlan): Promise<BuildResult>;
  planDeploy(project: ProjectConfig, artifact: BuildArtifact): Promise<DeployPlan>;
  deploy(project: ProjectConfig, plan: DeployPlan): Promise<DeployResult>;
  invoke?(input: InvokeInput): Promise<InvokeResult>;
  logs?(input: LogsInput): Promise<LogsResult>;
  destroy?(project: ProjectConfig): Promise<void>;
}
