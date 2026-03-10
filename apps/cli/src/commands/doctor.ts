import type { CommandRegistrar } from "../types/cli";
import { resolve } from "node:path";
import { evaluateCredentialSchema } from "@runfabric/core";
import { createProviderRegistry } from "../providers/registry";
import { loadPlanningContext } from "../utils/load-config";
import { printList } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info, success, warn } from "../utils/logger";

export const registerDoctorCommand: CommandRegistrar = (program) => {
  program
    .command("doctor")
    .description("Run environment and project diagnostics")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .action(async (options: { config?: string; stage?: string }) => {
      const majorNodeVersion = Number(process.versions.node.split(".")[0]);
      if (majorNodeVersion < 20) {
        error(`node ${process.versions.node} is too old; require >= 20`);
        process.exitCode = 1;
        return;
      }

      info(`node version ok: ${process.versions.node}`);

      const configPath = options.config ? resolve(process.cwd(), options.config) : undefined;
      const projectDir = await resolveProjectDir(process.cwd(), options.config);
      const context = await loadPlanningContext(projectDir, configPath, options.stage);
      const registry = createProviderRegistry(projectDir);

      const unsupportedProviders = context.project.providers.filter((provider) => !registry[provider]);
      if (unsupportedProviders.length > 0) {
        printList("Unsupported providers", unsupportedProviders);
        process.exitCode = 1;
        return;
      }

      if (context.planning.warnings.length > 0) {
        for (const warning of context.planning.warnings) {
          warn(warning);
        }
      }

      if (context.planning.errors.length > 0) {
        for (const planningError of context.planning.errors) {
          error(planningError);
        }
        process.exitCode = 1;
        return;
      }

      let hasCredentialErrors = false;
      for (const providerName of context.project.providers) {
        const provider = registry[providerName];
        if (!provider) {
          continue;
        }

        const credentialSchema = provider.getCredentialSchema?.();
        if (!credentialSchema) {
          warn(`${providerName}: credential schema is not defined; skipping credential check`);
          continue;
        }

        const credentialEvaluation = evaluateCredentialSchema(credentialSchema, process.env);
        for (const missing of credentialEvaluation.missingRequired) {
          error(`${providerName}: missing ${missing.env} (${missing.description})`);
          hasCredentialErrors = true;
        }
      }

      if (hasCredentialErrors) {
        process.exitCode = 1;
        return;
      }

      success("doctor checks passed");
    });
};
