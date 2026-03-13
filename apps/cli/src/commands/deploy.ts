import type { CommandRegistrar } from "../types/cli";
import type { Command } from "commander";
import { resolve } from "node:path";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info, warn } from "../utils/logger";
import {
  executeDeployWorkflow,
  type DeployWorkflowInput,
  type DeployWorkflowResult
} from "./deploy/workflow";

interface DeployCommandOptions {
  config?: string;
  stage?: string;
  out?: string;
  json?: boolean;
  rollbackOnFailure?: boolean;
  function?: string;
}

function logDeployResult(result: DeployWorkflowResult): void {
  info(`stage: ${result.stage}`);
  info(`deployed to ${result.deployments.length} provider(s)`);

  for (const deployment of result.deployments) {
    const modeSuffix = deployment.mode ? ` [${deployment.mode}]` : "";
    info(`${deployment.provider}${modeSuffix}: ${deployment.endpoint || "no endpoint"}`);
  }

  const simulatedProviders = result.deployments
    .filter((deployment) => deployment.mode === "simulated")
    .map((deployment) => deployment.provider);
  if (simulatedProviders.length > 0) {
    warn(
      `simulated deploy mode detected for ${simulatedProviders.join(
        ", "
      )}; no cloud resources were created. Configure real deploy env vars and commands (see docs/CREDENTIALS.md).`
    );
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
      if (rollback.status === "succeeded") {
        info(`${rollback.provider}: rollback succeeded`);
      } else if (rollback.status === "unsupported") {
        warn(`${rollback.provider}: rollback unsupported (${rollback.message || "destroy not available"})`);
      } else {
        warn(`${rollback.provider}: rollback failed (${rollback.message || "unknown"})`);
      }
    }
  }
}

async function runDeployCommand(
  functionName: string | undefined,
  options: DeployCommandOptions
): Promise<void> {
  const configPath = options.config ? resolve(process.cwd(), options.config) : undefined;
  const projectDir = await resolveProjectDir(process.cwd(), options.config);

  const workflowInput: DeployWorkflowInput = {
    projectDir,
    configPath,
    stage: options.stage,
    outputRoot: options.out,
    functionName,
    rollbackOnFailure: options.rollbackOnFailure,
    emitProgress: !options.json
  };
  const result = await executeDeployWorkflow(workflowInput);

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
}

function withRollbackOptions(command: Command): Command {
  return command
    .option("--rollback-on-failure", "Rollback successful providers when deploy has failures")
    .option("--no-rollback-on-failure", "Disable rollback when deploy has failures");
}

function registerNamedDeployCommand(command: Command): void {
  withRollbackOptions(
    command
      .option("-c, --config <path>", "Path to runfabric config")
      .option("-s, --stage <name>", "Stage name override")
      .option("-o, --out <path>", "Output directory")
      .option("--json", "Emit JSON output")
  ).action(async (name: string, options: DeployCommandOptions) => {
    await runDeployCommand(name, options);
  });
}

export const registerDeployCommand: CommandRegistrar = (program) => {
  const deployCommand = withRollbackOptions(
    program
      .command("deploy")
      .description("Deploy built artifacts to providers")
      .option("-c, --config <path>", "Path to runfabric config")
      .option("-s, --stage <name>", "Stage name override")
      .option("-f, --function <name>", "Deploy a specific function")
      .option("-o, --out <path>", "Output directory")
      .option("--json", "Emit JSON output")
  ).action(async (options: DeployCommandOptions) => {
    await runDeployCommand(options.function, options);
  });

  registerNamedDeployCommand(
    deployCommand
      .command("fn <name>")
      .alias("function")
      .description("Deploy a specific function from config")
  );
  registerNamedDeployCommand(
    program
      .command("deploy-function <name>")
      .description("Deploy a specific function from config (alias)")
  );
};

export { executeDeployWorkflow } from "./deploy/workflow";
export type { DeployWorkflowInput, DeployWorkflowResult } from "./deploy/workflow";
