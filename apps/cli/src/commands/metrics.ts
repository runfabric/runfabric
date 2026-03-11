import type { CommandRegistrar } from "../types/cli";
import { buildProviderMetricsFromLocalArtifacts } from "@runfabric/core";
import { createProviderRegistry, getProviderPackageName } from "../providers/registry";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info } from "../utils/logger";

export const registerMetricsCommand: CommandRegistrar = (program) => {
  program
    .command("metrics")
    .description("Fetch provider metrics derived from local deploy/invoke/log artifacts")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-p, --provider <name>", "Provider name", "aws-lambda")
    .option("--since <iso>", "Only include metrics based on logs since ISO timestamp")
    .option("--json", "Emit JSON output")
    .action(async (options: { config?: string; provider: string; since?: string; json?: boolean }) => {
      const projectDir = await resolveProjectDir(process.cwd(), options.config);
      const providers = createProviderRegistry(projectDir, [options.provider]);
      const provider = providers[options.provider];
      if (!provider) {
        const packageName = getProviderPackageName(options.provider);
        error(
          packageName
            ? `unknown provider: ${options.provider} (install ${packageName})`
            : `unknown provider: ${options.provider}`
        );
        process.exitCode = 1;
        return;
      }

      const result = provider.metrics
        ? await provider.metrics({
            provider: provider.name,
            since: options.since
          })
        : await buildProviderMetricsFromLocalArtifacts(projectDir, provider.name, {
            provider: provider.name,
            since: options.since
          });

      if (options.json) {
        printJson(result);
        return;
      }

      for (const metric of result.metrics) {
        info(`${provider.name}: ${metric.name}=${metric.value}${metric.unit ? ` ${metric.unit}` : ""}`);
      }
    });
};
