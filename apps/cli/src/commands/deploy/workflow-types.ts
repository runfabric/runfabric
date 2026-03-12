import type {
  BuildResult,
  DeployFailure,
  DeploymentMode,
  LifecycleHook,
  ProjectConfig,
  ProviderAdapter,
  StateAddress,
  StateBackend,
  StateLockInfo
} from "@runfabric/core";

export interface DeployWorkflowInput {
  projectDir: string;
  configPath?: string;
  stage?: string;
  outputRoot?: string;
  functionName?: string;
  emitProgress?: boolean;
}

export interface DeployWorkflowResult {
  stage: string;
  project: ProjectConfig;
  deployments: Array<{ provider: string; endpoint?: string; mode?: DeploymentMode }>;
  failures: DeployFailure[];
  rollbacks: Array<{ provider: string; ok: boolean; message?: string }>;
  summary: {
    targetedProviders: number;
    deployedProviders: number;
    failedProviders: number;
    rolledBackProviders: number;
    exitCode: number;
  };
}

export interface SuccessfulProviderDeployment {
  provider: string;
  adapter: ProviderAdapter;
}

export interface LockHeartbeat {
  stop(): Promise<void>;
  latestLock(): StateLockInfo;
}

export interface DeployCollections {
  deployments: Array<{ provider: string; endpoint?: string; mode?: DeploymentMode }>;
  failures: DeployFailure[];
  successfulDeployments: SuccessfulProviderDeployment[];
  rollbacks: Array<{ provider: string; ok: boolean; message?: string }>;
}

export interface DeployContext {
  stage: string;
  project: ProjectConfig;
  stateBackend: StateBackend;
  providerRegistry: Record<string, ProviderAdapter | undefined>;
  hooks: LifecycleHook[];
  buildResult: BuildResult;
}

export interface ProviderDeployInput {
  input: DeployWorkflowInput;
  context: DeployContext;
  artifact: BuildResult["artifacts"][number];
  collections: DeployCollections;
}

export interface ProviderStateSession {
  stateAddress: StateAddress;
  stateLock: StateLockInfo;
  heartbeat: LockHeartbeat;
  deploymentStartedAt: string;
}
