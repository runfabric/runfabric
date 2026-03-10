import type { CommandRegistrar } from "../types/cli";
import { dirname, resolve } from "node:path";
import { executeDeployWorkflow } from "./deploy";
import { loadPlanningContext } from "../utils/load-config";
import { loadComposeConfig, sortComposeServices, toComposeOutputEnvKey } from "../utils/compose";
import { printJson } from "../utils/output";
import { error, info } from "../utils/logger";

export const registerComposeCommand: CommandRegistrar = (program) => {
  const composeCommand = program
    .command("compose")
    .description("Compose-style orchestration for multiple runfabric services");

  composeCommand
    .command("plan")
    .description("Plan all compose services in dependency order")
    .option("-f, --file <path>", "Path to compose config", "runfabric.compose.yml")
    .option("-s, --stage <name>", "Stage name override")
    .option("--json", "Emit JSON output")
    .action(async (options: { file: string; stage?: string; json?: boolean }) => {
      const composePath = resolve(process.cwd(), options.file);
      const compose = await loadComposeConfig(composePath);
      const order = sortComposeServices(compose);
      const services: Array<{ name: string; config: string; ok: boolean; errors: string[] }> = [];

      for (const service of order) {
        const projectDir = dirname(service.config);
        const planning = await loadPlanningContext(projectDir, service.config, options.stage);
        services.push({
          name: service.name,
          config: service.config,
          ok: planning.planning.ok,
          errors: planning.planning.errors
        });
      }

      const payload = {
        compose: composePath,
        stage: options.stage || "default",
        order: order.map((service) => service.name),
        services
      };

      if (!services.every((service) => service.ok)) {
        process.exitCode = 1;
      }

      if (options.json) {
        printJson(payload);
      } else {
        info(`compose plan order: ${payload.order.join(" -> ")}`);
        for (const service of services) {
          info(`${service.name}: ${service.ok ? "ok" : "failed"}`);
          for (const planningError of service.errors) {
            error(`  ${planningError}`);
          }
        }
      }
    });

  composeCommand
    .command("deploy")
    .description("Deploy compose services with cross-service output sharing")
    .option("-f, --file <path>", "Path to compose config", "runfabric.compose.yml")
    .option("-s, --stage <name>", "Stage name override")
    .option("--json", "Emit JSON output")
    .action(async (options: { file: string; stage?: string; json?: boolean }) => {
      const composePath = resolve(process.cwd(), options.file);
      const compose = await loadComposeConfig(composePath);
      const order = sortComposeServices(compose);
      const serviceResults: Array<{
        name: string;
        config: string;
        summary: { exitCode: number; deployedProviders: number; failedProviders: number };
      }> = [];
      const sharedOutputs: Record<string, string> = {};

      for (const service of order) {
        const projectDir = dirname(service.config);
        const result = await executeDeployWorkflow({
          projectDir,
          configPath: service.config,
          stage: options.stage
        });

        serviceResults.push({
          name: service.name,
          config: service.config,
          summary: {
            exitCode: result.summary.exitCode,
            deployedProviders: result.summary.deployedProviders,
            failedProviders: result.summary.failedProviders
          }
        });

        for (const deployment of result.deployments) {
          if (!deployment.endpoint) {
            continue;
          }
          const key = toComposeOutputEnvKey(service.name, deployment.provider);
          process.env[key] = deployment.endpoint;
          sharedOutputs[key] = deployment.endpoint;
        }

        if (result.summary.exitCode !== 0) {
          process.exitCode = result.summary.exitCode;
          break;
        }
      }

      const payload = {
        compose: composePath,
        stage: options.stage || "default",
        order: order.map((service) => service.name),
        services: serviceResults,
        sharedOutputs
      };

      if (options.json) {
        printJson(payload);
      } else {
        info(`compose deploy order: ${payload.order.join(" -> ")}`);
        for (const service of serviceResults) {
          info(
            `${service.name}: deployed=${service.summary.deployedProviders} failed=${service.summary.failedProviders} exit=${service.summary.exitCode}`
          );
        }
        info(`shared outputs exported: ${Object.keys(sharedOutputs).length}`);
      }
    });
};

