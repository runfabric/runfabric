import { buildProject } from "@runfabric/builder";
import {
  createStateBackend,
  type DeployFailure,
  type DeploymentMode
} from "@runfabric/core";
import { createPlan } from "@runfabric/planner";
import { createProviderRegistry } from "../../providers/registry";
import { loadPlanningContext } from "../../utils/load-config";
import { loadLifecycleHooks } from "../../utils/hooks";
import { resolveFunctionProject } from "../../utils/project-functions";
import { info } from "../../utils/logger";
import {
  createDeployCollections,
  deploySingleArtifact,
  rollbackDeployments
} from "./workflow-provider";
import type {
  DeployCollections,
  DeployContext,
  DeployRollbackResult,
  DeployWorkflowInput,
  DeployWorkflowResult
} from "./workflow-types";

function logProgress(enabled: boolean | undefined, message: string): void {
  if (enabled) {
    info(message);
  }
}

function summarizeResult(
  targetedProviders: number,
  deployments: Array<{ provider: string; endpoint?: string; mode?: DeploymentMode }>,
  failures: DeployFailure[],
  rollbacks: DeployRollbackResult[]
): DeployWorkflowResult["summary"] {
  const exitCode = failures.length === 0 ? 0 : deployments.length > 0 ? 2 : 1;
  return {
    targetedProviders,
    deployedProviders: deployments.length,
    failedProviders: failures.length,
    rolledBackProviders: rollbacks.filter((rollback) => rollback.ok).length,
    exitCode
  };
}

function createPlanningFailureResult(
  project: DeployWorkflowResult["project"],
  planningErrors: string[]
): DeployWorkflowResult {
  const failures: DeployFailure[] = planningErrors.map((planningError) => ({
    provider: "planner",
    phase: "deploy",
    message: planningError
  }));
  const rollbacks: DeployRollbackResult[] = [];

  return {
    stage: project.stage || "default",
    project,
    deployments: [],
    failures,
    rollbacks,
    summary: summarizeResult(0, [], failures, rollbacks)
  };
}

async function runBeforeBuildHooks(
  context: Omit<DeployContext, "buildResult">,
  input: DeployWorkflowInput
): Promise<void> {
  for (const hook of context.hooks) {
    await hook.beforeBuild?.({
      project: context.project,
      projectDir: input.projectDir,
      outputRoot: input.outputRoot
    });
  }
}

async function runAfterBuildHooks(
  context: DeployContext,
  input: DeployWorkflowInput
): Promise<void> {
  for (const hook of context.hooks) {
    await hook.afterBuild?.({
      project: context.project,
      projectDir: input.projectDir,
      outputRoot: input.outputRoot,
      artifacts: context.buildResult.artifacts
    });
  }
}

async function runBeforeDeployHooks(
  context: DeployContext,
  input: DeployWorkflowInput
): Promise<void> {
  for (const hook of context.hooks) {
    await hook.beforeDeploy?.({
      project: context.project,
      projectDir: input.projectDir,
      stage: context.stage,
      outputRoot: input.outputRoot,
      functionName: input.functionName
    });
  }
}

async function runAfterDeployHooks(
  context: DeployContext,
  input: DeployWorkflowInput,
  collections: DeployCollections,
  exitCode: number
): Promise<void> {
  for (const hook of context.hooks) {
    await hook.afterDeploy?.({
      project: context.project,
      projectDir: input.projectDir,
      stage: context.stage,
      outputRoot: input.outputRoot,
      functionName: input.functionName,
      deployments: collections.deployments,
      failures: collections.failures,
      exitCode
    });
  }
}

function isDeployWorkflowResult(value: DeployContext | DeployWorkflowResult): value is DeployWorkflowResult {
  return "summary" in value;
}

async function createDeployContext(
  input: DeployWorkflowInput
): Promise<DeployContext | DeployWorkflowResult> {
  logProgress(input.emitProgress, "deploy: loading project and planning");
  const planningContext = await loadPlanningContext(input.projectDir, input.configPath, input.stage);
  const project = resolveFunctionProject(planningContext.project, input.functionName);
  const planning = project === planningContext.project ? planningContext.planning : createPlan(project);

  if (!planning.ok) {
    return createPlanningFailureResult(project, planning.errors);
  }

  const providerRegistry = createProviderRegistry(input.projectDir, project.providers);
  const stateBackend = createStateBackend({ projectDir: input.projectDir, state: project.state });
  const hooks = await loadLifecycleHooks(project, input.projectDir);
  const contextWithoutBuild: Omit<DeployContext, "buildResult"> = {
    stage: project.stage || "default",
    project,
    stateBackend,
    providerRegistry,
    hooks
  };

  logProgress(input.emitProgress, "deploy: running beforeBuild hooks");
  await runBeforeBuildHooks(contextWithoutBuild, input);

  const buildResult = await buildProject({
    planning,
    project,
    projectDir: input.projectDir,
    outputRoot: input.outputRoot
  });
  logProgress(input.emitProgress, `deploy: build complete (${buildResult.artifacts.length} provider artifact(s))`);

  const context: DeployContext = { ...contextWithoutBuild, buildResult };
  logProgress(input.emitProgress, "deploy: running afterBuild hooks");
  await runAfterBuildHooks(context, input);
  return context;
}

export async function executeDeployWorkflow(
  input: DeployWorkflowInput
): Promise<DeployWorkflowResult> {
  const loaded = await createDeployContext(input);
  if (isDeployWorkflowResult(loaded)) {
    return loaded;
  }

  const context = loaded;
  const collections = createDeployCollections();

  logProgress(input.emitProgress, `deploy: stage=${context.stage} backend=${context.stateBackend.config.backend}`);
  logProgress(input.emitProgress, "deploy: running beforeDeploy hooks");
  await runBeforeDeployHooks(context, input);

  for (const artifact of context.buildResult.artifacts) {
    await deploySingleArtifact({ input, context, artifact, collections });
  }

  await rollbackDeployments(input, context, collections);
  const summary = summarizeResult(
    context.buildResult.artifacts.length,
    collections.deployments,
    collections.failures,
    collections.rollbacks
  );

  await runAfterDeployHooks(context, input, collections, summary.exitCode);
  logProgress(input.emitProgress, "deploy: finished");

  return {
    stage: context.stage,
    project: context.project,
    deployments: collections.deployments,
    failures: collections.failures,
    rollbacks: collections.rollbacks,
    summary
  };
}

export type { DeployWorkflowInput, DeployWorkflowResult };
