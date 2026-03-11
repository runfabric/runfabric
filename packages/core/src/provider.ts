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
  resourceAddresses?: Record<string, string>;
  workflowAddresses?: Record<string, string>;
  secretReferences?: Record<string, string>;
}

export interface InvokeInput {
  provider: string;
  payload?: string;
}

export interface InvokeResult {
  statusCode: number;
  body?: string;
  correlation?: {
    deploymentId?: string;
    invokeId?: string;
  };
}

export interface LogsInput {
  provider: string;
  since?: string;
}

export interface LogsResult {
  lines: string[];
}

export interface TracesInput {
  provider: string;
  since?: string;
  correlationId?: string;
  limit?: number;
}

export interface TraceRecord {
  timestamp: string;
  provider: string;
  message: string;
  deploymentId?: string;
  invokeId?: string;
  correlationId?: string;
}

export interface TracesResult {
  traces: TraceRecord[];
}

export interface MetricsInput {
  provider: string;
  since?: string;
}

export interface MetricsResult {
  metrics: Array<{
    name: string;
    value: number;
    unit?: string;
  }>;
}

export interface ResourceProvisionResult {
  provider: string;
  resourceAddresses: Record<string, string>;
}

export interface WorkflowDeployResult {
  provider: string;
  workflowAddresses: Record<string, string>;
}

export interface SecretMaterializationResult {
  provider: string;
  secretReferences: Record<string, string>;
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
  provisionResources?(project: ProjectConfig): Promise<ResourceProvisionResult>;
  deployWorkflows?(project: ProjectConfig): Promise<WorkflowDeployResult>;
  materializeSecrets?(project: ProjectConfig): Promise<SecretMaterializationResult>;
  invoke?(input: InvokeInput): Promise<InvokeResult>;
  logs?(input: LogsInput): Promise<LogsResult>;
  traces?(input: TracesInput): Promise<TracesResult>;
  metrics?(input: MetricsInput): Promise<MetricsResult>;
  destroy?(project: ProjectConfig): Promise<void>;
}
