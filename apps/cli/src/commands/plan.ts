import type { CommandRegistrar } from "../types/cli";
import { resolve } from "node:path";
import { loadPlanningContext } from "../utils/load-config";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info, warn } from "../utils/logger";

export const registerPlanCommand: CommandRegistrar = (program) => {
  program
    .command("plan")
    .description("Generate a provider-aware deployment plan")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-s, --stage <name>", "Stage name override")
    .option("--json", "Emit JSON output")
    .action(async (options: { config?: string; stage?: string; json?: boolean }) => {
      const configPath = options.config ? resolve(process.cwd(), options.config) : undefined;
      const projectDir = await resolveProjectDir(process.cwd(), options.config);
      const context = await loadPlanningContext(projectDir, configPath, options.stage);

      if (options.json) {
        printJson(context.planning);
      } else {
        info(`service: ${context.project.service}`);
        info(`stage: ${context.project.stage || "default"}`);
        info(
          `portability: universal triggers = ${
            context.planning.portability.universallySupportedTriggerTypes.join(", ") || "none"
          }`
        );
        info(
          `primitives: universal = ${
            context.planning.primitiveCompatibility.universallySupported.join(", ") || "none"
          }`
        );
        info(
          `primitives: partial = ${
            context.planning.primitiveCompatibility.partiallySupported.join(", ") || "none"
          }`
        );
        for (const providerPlan of context.planning.providerPlans) {
          const status = providerPlan.errors.length === 0 ? "ok" : "failed";
          info(`${providerPlan.provider}: ${status}`);
          for (const providerWarning of providerPlan.warnings) {
            warn(`  warning: ${providerWarning}`);
          }
          for (const providerError of providerPlan.errors) {
            error(`  error: ${providerError}`);
          }
        }
      }

      if (!context.planning.ok) {
        process.exitCode = 1;
      }
    });
};
