import type { CommandRegistrar } from "../types/cli";
import { dirname, resolve } from "node:path";
import { executeDeployWorkflow } from "./deploy";
import { executeRemoveWorkflow } from "./remove";
import { loadPlanningContext } from "../utils/load-config";
import {
  composeServiceLevels,
  loadComposeConfig,
  sortComposeServices,
  toComposeOutputEnvKey,
  type ComposeServiceConfig
} from "../utils/compose";
import { printJson } from "../utils/output";
import { error, info } from "../utils/logger";

const DEFAULT_COMPOSE_CONCURRENCY = 4;

interface ComposeOptions {
  file: string;
  stage?: string;
  json?: boolean;
  rollbackOnFailure?: boolean;
  provider?: string;
  concurrency: number;
}

interface ComposeDeployServiceResult {
  name: string;
  config: string;
  summary: { exitCode: number; deployedProviders: number; failedProviders: number };
}

interface ComposeRemoveServiceResult {
  name: string;
  config: string;
  summary: { exitCode: number; destroyedProviders: number; failures: number };
}

function parseConcurrency(value: string): number {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isInteger(parsed) || parsed <= 0 || parsed > 32) {
    throw new Error(`invalid concurrency: ${value}. expected integer between 1 and 32`);
  }
  return parsed;
}

async function runBoundedConcurrent<T, R>(
  items: readonly T[],
  limit: number,
  worker: (item: T) => Promise<R>
): Promise<R[]> {
  if (items.length === 0) {
    return [];
  }

  const results: R[] = new Array(items.length);
  let cursor = 0;
  const workerCount = Math.min(Math.max(limit, 1), items.length);
  await Promise.all(
    Array.from({ length: workerCount }, async () => {
      while (cursor < items.length) {
        const current = cursor;
        cursor += 1;
        results[current] = await worker(items[current]);
      }
    })
  );
  return results;
}

function printComposePlan(
  payload: {
    compose: string;
    stage: string;
    concurrency: number;
    order: string[];
    services: Array<{ name: string; config: string; ok: boolean; errors: string[] }>;
  },
  json: boolean | undefined
): void {
  if (json) {
    printJson(payload);
    return;
  }

  info(`compose plan order: ${payload.order.join(" -> ")}`);
  info(`compose plan concurrency: ${payload.concurrency}`);
  for (const service of payload.services) {
    info(`${service.name}: ${service.ok ? "ok" : "failed"}`);
    for (const planningError of service.errors) {
      error(`  ${planningError}`);
    }
  }
}

function printComposeDeploy(
  payload: {
    compose: string;
    stage: string;
    concurrency: number;
    order: string[];
    services: ComposeDeployServiceResult[];
    sharedOutputs: Record<string, string>;
  },
  json: boolean | undefined
): void {
  if (json) {
    printJson(payload);
    return;
  }

  info(`compose deploy order: ${payload.order.join(" -> ")}`);
  info(`compose deploy concurrency: ${payload.concurrency}`);
  for (const service of payload.services) {
    info(
      `${service.name}: deployed=${service.summary.deployedProviders} failed=${service.summary.failedProviders} exit=${service.summary.exitCode}`
    );
  }
  info(`shared outputs exported: ${Object.keys(payload.sharedOutputs).length}`);
}

function printComposeRemove(
  payload: {
    compose: string;
    stage: string;
    concurrency: number;
    order: string[];
    services: ComposeRemoveServiceResult[];
  },
  json: boolean | undefined
): void {
  if (json) {
    printJson(payload);
    return;
  }

  info(`compose remove order: ${payload.order.join(" -> ")}`);
  info(`compose remove concurrency: ${payload.concurrency}`);
  for (const service of payload.services) {
    info(
      `${service.name}: destroyed=${service.summary.destroyedProviders} failures=${service.summary.failures} exit=${service.summary.exitCode}`
    );
  }
}

async function executeComposePlan(options: ComposeOptions): Promise<void> {
  const composePath = resolve(process.cwd(), options.file);
  const compose = await loadComposeConfig(composePath);
  const levels = composeServiceLevels(compose);
  const order = sortComposeServices(compose);
  const services: Array<{ name: string; config: string; ok: boolean; errors: string[] }> = [];

  for (const level of levels) {
    const levelResults = await runBoundedConcurrent(level, options.concurrency, async (service) => {
      const projectDir = dirname(service.config);
      const planning = await loadPlanningContext(projectDir, service.config, options.stage);
      return {
        name: service.name,
        config: service.config,
        ok: planning.planning.ok,
        errors: planning.planning.errors
      };
    });
    services.push(...levelResults);
  }

  const payload = {
    compose: composePath,
    stage: options.stage || "default",
    concurrency: options.concurrency,
    order: order.map((service) => service.name),
    services
  };
  if (!services.every((service) => service.ok)) {
    process.exitCode = 1;
  }
  printComposePlan(payload, options.json);
}

async function deployComposeService(
  service: ComposeServiceConfig,
  stage: string | undefined,
  rollbackOnFailure: boolean | undefined
): Promise<{ result: ComposeDeployServiceResult; outputs: Record<string, string> }> {
  const projectDir = dirname(service.config);
  const deployResult = await executeDeployWorkflow({
    projectDir,
    configPath: service.config,
    stage,
    rollbackOnFailure
  });

  const outputs: Record<string, string> = {};
  for (const deployment of deployResult.deployments) {
    if (!deployment.endpoint) {
      continue;
    }
    const key = toComposeOutputEnvKey(service.name, deployment.provider);
    outputs[key] = deployment.endpoint;
  }

  return {
    result: {
      name: service.name,
      config: service.config,
      summary: {
        exitCode: deployResult.summary.exitCode,
        deployedProviders: deployResult.summary.deployedProviders,
        failedProviders: deployResult.summary.failedProviders
      }
    },
    outputs
  };
}

async function executeComposeDeploy(options: ComposeOptions): Promise<void> {
  const composePath = resolve(process.cwd(), options.file);
  const compose = await loadComposeConfig(composePath);
  const levels = composeServiceLevels(compose);
  const order = sortComposeServices(compose);
  const services: ComposeDeployServiceResult[] = [];
  const sharedOutputs: Record<string, string> = {};
  let exitCode = 0;

  for (const level of levels) {
    if (options.concurrency === 1) {
      for (const service of level) {
        const item = await deployComposeService(service, options.stage, options.rollbackOnFailure);
        services.push(item.result);
        for (const [key, value] of Object.entries(item.outputs)) {
          process.env[key] = value;
          sharedOutputs[key] = value;
        }
        if (item.result.summary.exitCode !== 0 && exitCode === 0) {
          exitCode = item.result.summary.exitCode;
          break;
        }
      }
    } else {
      const levelResults = await runBoundedConcurrent(level, options.concurrency, async (service) =>
        deployComposeService(service, options.stage, options.rollbackOnFailure)
      );
      for (const item of levelResults) {
        services.push(item.result);
        for (const [key, value] of Object.entries(item.outputs)) {
          process.env[key] = value;
          sharedOutputs[key] = value;
        }
        if (item.result.summary.exitCode !== 0 && exitCode === 0) {
          exitCode = item.result.summary.exitCode;
        }
      }
    }

    if (exitCode !== 0) {
      break;
    }
  }

  if (exitCode !== 0) {
    process.exitCode = exitCode;
  }

  const payload = {
    compose: composePath,
    stage: options.stage || "default",
    concurrency: options.concurrency,
    order: order.map((service) => service.name),
    services,
    sharedOutputs
  };
  printComposeDeploy(payload, options.json);
}

async function removeComposeService(
  service: ComposeServiceConfig,
  stage: string | undefined,
  provider: string | undefined
): Promise<ComposeRemoveServiceResult> {
  const projectDir = dirname(service.config);
  const removeResult = await executeRemoveWorkflow({
    projectDir,
    configPath: service.config,
    stage,
    provider
  });
  return {
    name: service.name,
    config: service.config,
    summary: {
      exitCode: removeResult.summary.exitCode,
      destroyedProviders: removeResult.destroyedProviders.length,
      failures: removeResult.failures.length
    }
  };
}

async function executeComposeRemove(options: ComposeOptions): Promise<void> {
  const composePath = resolve(process.cwd(), options.file);
  const compose = await loadComposeConfig(composePath);
  const levels = composeServiceLevels(compose);
  const reverseLevels = [...levels].reverse();
  const services: ComposeRemoveServiceResult[] = [];
  const order = reverseLevels.flat().map((service) => service.name);
  let exitCode = 0;

  for (const level of reverseLevels) {
    const levelResults = await runBoundedConcurrent(level, options.concurrency, async (service) =>
      removeComposeService(service, options.stage, options.provider)
    );
    services.push(...levelResults);
    if (levelResults.some((service) => service.summary.exitCode !== 0)) {
      exitCode = 1;
    }
  }

  if (exitCode !== 0) {
    process.exitCode = exitCode;
  }

  const payload = {
    compose: composePath,
    stage: options.stage || "default",
    concurrency: options.concurrency,
    order,
    services
  };
  printComposeRemove(payload, options.json);
}

export const registerComposeCommand: CommandRegistrar = (program) => {
  const composeCommand = program
    .command("compose")
    .description("Compose-style orchestration for multiple runfabric services");

  composeCommand
    .command("plan")
    .description("Plan all compose services in dependency order")
    .option("-f, --file <path>", "Path to compose config", "runfabric.compose.yml")
    .option("-s, --stage <name>", "Stage name override")
    .option("--concurrency <number>", "Max in-flight independent services (1-32)", parseConcurrency, DEFAULT_COMPOSE_CONCURRENCY)
    .option("--json", "Emit JSON output")
    .action(async (options: ComposeOptions) => executeComposePlan(options));

  composeCommand
    .command("deploy")
    .description("Deploy compose services with cross-service output sharing")
    .option("-f, --file <path>", "Path to compose config", "runfabric.compose.yml")
    .option("-s, --stage <name>", "Stage name override")
    .option("--rollback-on-failure", "Rollback successful providers when deploy has failures")
    .option("--no-rollback-on-failure", "Disable rollback when deploy has failures")
    .option("--concurrency <number>", "Max in-flight independent services (1-32)", parseConcurrency, DEFAULT_COMPOSE_CONCURRENCY)
    .option("--json", "Emit JSON output")
    .action(async (options: ComposeOptions) => executeComposeDeploy(options));

  composeCommand
    .command("remove")
    .description("Remove compose services in reverse dependency order")
    .option("-f, --file <path>", "Path to compose config", "runfabric.compose.yml")
    .option("-s, --stage <name>", "Stage name override")
    .option("-p, --provider <name>", "Limit removal to a single provider for each service")
    .option("--concurrency <number>", "Max in-flight independent services (1-32)", parseConcurrency, DEFAULT_COMPOSE_CONCURRENCY)
    .option("--json", "Emit JSON output")
    .action(async (options: ComposeOptions) => executeComposeRemove(options));
};
