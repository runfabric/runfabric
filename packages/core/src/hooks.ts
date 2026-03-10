import type { BuildArtifact } from "./provider";
import type { ProjectConfig } from "./project";

export interface BuildHookContext {
  project: ProjectConfig;
  projectDir: string;
  outputRoot?: string;
  artifacts?: BuildArtifact[];
}

export interface DeployFailure {
  provider: string;
  phase: "provider" | "validation" | "deploy" | "state" | "rollback";
  message: string;
}

export interface DeployHookContext {
  project: ProjectConfig;
  projectDir: string;
  stage: string;
  outputRoot?: string;
  functionName?: string;
  deployments?: Array<{ provider: string; endpoint?: string }>;
  failures?: DeployFailure[];
  exitCode?: number;
}

export interface LifecycleHook {
  name?: string;
  beforeBuild?(context: BuildHookContext): Promise<void> | void;
  afterBuild?(context: BuildHookContext): Promise<void> | void;
  beforeDeploy?(context: DeployHookContext): Promise<void> | void;
  afterDeploy?(context: DeployHookContext): Promise<void> | void;
}
