import type { CommandRegistrar } from "../types/cli";
import { rm } from "node:fs/promises";
import { resolve } from "node:path";
import { buildProject } from "@runfabric/builder";
import {
  LocalFileStateBackend,
  type DeployFailure,
  type ProjectConfig,
  type ProviderAdapter
} from "@runfabric/core";
import { createPlan } from "@runfabric/planner";
import { createProviderRegistry } from "../providers/registry";
import { loadPlanningContext } from "../utils/load-config";
import { loadLifecycleHooks } from "../utils/hooks";
import { resolveFunctionProject } from "../utils/project-functions";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info, warn } from "../utils/logger";

export interface DeployWorkflowInput {
  projectDir: string;
  configPath?: string;
  stage?: string;
  outputRoot?: string;
  functionName?: string;
}

export interface DeployWorkflowResult {
  stage: string;
  project: ProjectConfig;
  deployments: Array<{ provider: string; endpoint?: string }>;
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

interface SuccessfulProviderDeployment {
  provider: string;
  adapter: ProviderAdapter;
}

function stringifyError(errorValue: unknown): string {
  if (errorValue instanceof Error) {
    return errorValue.message;
  }
  return String(errorValue);
}

function summarizeResult(
  targetedProviders: number,
  deployments: Array<{ provider: string; endpoint?: string }>,
  failures: DeployFailure[],
  rollbacks: Array<{ provider: string; ok: boolean; message?: string }>
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

function rollbackEnabled(): boolean {
  const configured = process.env.RUNFABRIC_ROLLBACK_ON_FAILURE;
  if (!configured) {
    return false;
  }
  const normalized = configured.trim().toLowerCase();
  if (!normalized) {
    return false;
  }
  return ["1", "true", "yes", "on"].includes(normalized);
}

export async function executeDeployWorkflow(
  input: DeployWorkflowInput
): Promise<DeployWorkflowResult> {
  const context = await loadPlanningContext(input.projectDir, input.configPath, input.stage);
  const baseProject = resolveFunctionProject(context.project, input.functionName);
  const planning = baseProject === context.project ? context.planning : createPlan(baseProject);

  if (!planning.ok) {
    const failures: DeployFailure[] = planning.errors.map((planningError) => ({
      provider: "planner",
      phase: "deploy",
      message: planningError
    }));
    const rollbacks: Array<{ provider: string; ok: boolean; message?: string }> = [];
    return {
      stage: baseProject.stage || "default",
      project: baseProject,
      deployments: [],
      failures,
      rollbacks,
      summary: summarizeResult(0, [], failures, rollbacks)
    };
  }

  const providerRegistry = createProviderRegistry(input.projectDir);
  const stateBackend = new LocalFileStateBackend({ projectDir: input.projectDir });
  const hooks = await loadLifecycleHooks(baseProject, input.projectDir);

  for (const hook of hooks) {
    await hook.beforeBuild?.({
      project: baseProject,
      projectDir: input.projectDir,
      outputRoot: input.outputRoot
    });
  }

  const buildResult = await buildProject({
    planning,
    project: baseProject,
    projectDir: input.projectDir,
    outputRoot: input.outputRoot
  });

  for (const hook of hooks) {
    await hook.afterBuild?.({
      project: baseProject,
      projectDir: input.projectDir,
      outputRoot: input.outputRoot,
      artifacts: buildResult.artifacts
    });
  }

  const deployments: Array<{ provider: string; endpoint?: string }> = [];
  const failures: DeployFailure[] = [];
  const rollbacks: Array<{ provider: string; ok: boolean; message?: string }> = [];
  const successfulDeployments: SuccessfulProviderDeployment[] = [];
  const stage = baseProject.stage || "default";

  for (const hook of hooks) {
    await hook.beforeDeploy?.({
      project: baseProject,
      projectDir: input.projectDir,
      stage,
      outputRoot: input.outputRoot,
      functionName: input.functionName
    });
  }

  for (const artifact of buildResult.artifacts) {
    const provider = providerRegistry[artifact.provider];
    if (!provider) {
      failures.push({
        provider: artifact.provider,
        phase: "provider",
        message: "provider adapter is not installed"
      });
      continue;
    }

    const validation = await provider.validate(baseProject);
    if (!validation.ok) {
      for (const providerError of validation.errors || []) {
        failures.push({
          provider: provider.name,
          phase: "validation",
          message: providerError
        });
      }
      continue;
    }

    try {
      const providerBuildPlan = await provider.planBuild(baseProject);
      await provider.build(baseProject, providerBuildPlan);
      const deployPlan = await provider.planDeploy(baseProject, artifact);
      const deployResult = await provider.deploy(baseProject, deployPlan);
      deployments.push(deployResult);
      successfulDeployments.push({
        provider: provider.name,
        adapter: provider
      });

      const stateAddress = {
        service: baseProject.service,
        stage,
        provider: provider.name
      };
      const stateRecord = {
        schemaVersion: 1,
        provider: provider.name,
        service: baseProject.service,
        stage,
        endpoint: deployResult.endpoint,
        updatedAt: new Date().toISOString(),
        details: {
          artifact,
          deployPlan,
          functionName: input.functionName
        }
      };

      try {
        await stateBackend.lock(stateAddress);
        try {
          await stateBackend.write(stateAddress, stateRecord);
        } finally {
          await stateBackend.unlock(stateAddress);
        }
      } catch (stateError) {
        failures.push({
          provider: provider.name,
          phase: "state",
          message: stringifyError(stateError)
        });
      }
    } catch (deployError) {
      failures.push({
        provider: provider.name,
        phase: "deploy",
        message: stringifyError(deployError)
      });
    }
  }

  if (failures.length > 0 && successfulDeployments.length > 0 && rollbackEnabled()) {
    for (const deployedProvider of [...successfulDeployments].reverse()) {
      const cleanupTargets = [
        resolve(input.projectDir, ".runfabric", "deploy", deployedProvider.provider),
        resolve(
          input.projectDir,
          ".runfabric",
          "state",
          baseProject.service,
          stage,
          `${deployedProvider.provider}.state.json`
        ),
        resolve(
          input.projectDir,
          ".runfabric",
          "state",
          baseProject.service,
          stage,
          `${deployedProvider.provider}.state.json.lock`
        )
      ];

      try {
        if (!deployedProvider.adapter.destroy) {
          throw new Error("provider destroy is not implemented");
        }

        await deployedProvider.adapter.destroy(baseProject);
        for (const cleanupTarget of cleanupTargets) {
          await rm(cleanupTarget, { recursive: true, force: true });
        }

        const deploymentIndex = deployments.findIndex((item) => item.provider === deployedProvider.provider);
        if (deploymentIndex >= 0) {
          deployments.splice(deploymentIndex, 1);
        }

        rollbacks.push({
          provider: deployedProvider.provider,
          ok: true
        });
      } catch (rollbackError) {
        const message = stringifyError(rollbackError);
        rollbacks.push({
          provider: deployedProvider.provider,
          ok: false,
          message
        });
        failures.push({
          provider: deployedProvider.provider,
          phase: "rollback",
          message
        });
      }
    }
  }

  const summary = summarizeResult(buildResult.artifacts.length, deployments, failures, rollbacks);

  for (const hook of hooks) {
    await hook.afterDeploy?.({
      project: baseProject,
      projectDir: input.projectDir,
      stage,
      outputRoot: input.outputRoot,
      functionName: input.functionName,
      deployments,
      failures,
      exitCode: summary.exitCode
    });
  }

  return {
    stage,
    project: baseProject,
    deployments,
    failures,
    rollbacks,
    summary
  };
}

function logDeployResult(result: DeployWorkflowResult): void {
  info(`stage: ${result.stage}`);
  info(`deployed to ${result.deployments.length} provider(s)`);
  for (const deployment of result.deployments) {
    info(`${deployment.provider}: ${deployment.endpoint || "no endpoint"}`);
  }

  if (result.failures.length > 0) {
    warn(`failed providers: ${result.failures.length}`);
    for (const failure of result.failures) {
      warn(`${failure.provider} [${failure.phase}]: ${failure.message}`);
    }
    if (result.summary.exitCode === 2) {
      warn("deploy completed with partial failures (exit code 2)");
    }
  }

  if (result.rollbacks.length > 0) {
    info(`rollback actions: ${result.rollbacks.length}`);
    for (const rollback of result.rollbacks) {
      if (rollback.ok) {
        info(`${rollback.provider}: rollback succeeded`);
      } else {
        warn(`${rollback.provider}: rollback failed (${rollback.message || "unknown"})`);
      }
    }
  }
}

export const registerDeployCommand: CommandRegistrar = (program) => {
  const runAndEmit = async (
    functionName: string | undefined,
    options: { config?: string; stage?: string; out?: string; json?: boolean }
  ): Promise<void> => {
    const configPath = options.config ? resolve(process.cwd(), options.config) : undefined;
    const projectDir = await resolveProjectDir(process.cwd(), options.config);

    const result = await executeDeployWorkflow({
      projectDir,
      configPath,
      stage: options.stage,
      outputRoot: options.out,
      functionName
    });

    if (result.summary.exitCode !== 0) {
      for (const failure of result.failures) {
        error(`${failure.provider}: ${failure.message}`);
      }
      process.exitCode = result.summary.exitCode;
    }

    if (options.json) {
      printJson(result);
    } else {
      logDeployResult(result);
    }
  };

  const deployCommand = program
    .command("deploy")
    .description("Deploy built artifacts to providers")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .option("-o, --out <path>", "Output directory")
    .option("--json", "Emit JSON output")
    .action(async (options: { config?: string; stage?: string; out?: string; json?: boolean }) => {
      await runAndEmit(undefined, options);
    });

  deployCommand
    .command("fn <name>")
    .alias("function")
    .description("Deploy a specific function from config")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .option("-o, --out <path>", "Output directory")
    .option("--json", "Emit JSON output")
    .action(async (name: string, options: { config?: string; stage?: string; out?: string; json?: boolean }) => {
      await runAndEmit(name, options);
    });

  program
    .command("deploy-function <name>")
    .description("Deploy a specific function from config (alias)")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .option("-o, --out <path>", "Output directory")
    .option("--json", "Emit JSON output")
    .action(async (name: string, options: { config?: string; stage?: string; out?: string; json?: boolean }) => {
      await runAndEmit(name, options);
    });
};
