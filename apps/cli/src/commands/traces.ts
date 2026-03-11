import type { CommandRegistrar } from "../types/cli";
import { buildProviderTracesFromLocalArtifacts } from "@runfabric/core";
import { createProviderRegistry } from "../providers/registry";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info } from "../utils/logger";

export const registerTracesCommand: CommandRegistrar = (program) => {
  program
    .command("traces")
    .description("Fetch provider trace records from local deploy/invoke/log artifacts")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-p, --provider <name>", "Provider name", "aws-lambda")
    .option("--since <iso>", "Only include traces since ISO timestamp")
    .option("--correlation-id <id>", "Filter traces by correlation ID")
    .option("--limit <count>", "Limit trace rows", (value: string) => Number.parseInt(value, 10))
    .option("--json", "Emit JSON output")
    .action(
      async (options: {
        config?: string;
        provider: string;
        since?: string;
        correlationId?: string;
        limit?: number;
        json?: boolean;
      }) => {
        const projectDir = await resolveProjectDir(process.cwd(), options.config);
        const providers = createProviderRegistry(projectDir);
        const provider = providers[options.provider];
        if (!provider) {
          error(`unknown provider: ${options.provider}`);
          process.exitCode = 1;
          return;
        }

        const result = provider.traces
          ? await provider.traces({
              provider: provider.name,
              since: options.since,
              correlationId: options.correlationId,
              limit: options.limit
            })
          : await buildProviderTracesFromLocalArtifacts(projectDir, provider.name, {
              provider: provider.name,
              since: options.since,
              correlationId: options.correlationId,
              limit: options.limit
            });

        if (options.json) {
          printJson(result);
          return;
        }

        if (result.traces.length === 0) {
          info(`${provider.name}: no traces found`);
          return;
        }
        for (const trace of result.traces) {
          info(
            `${trace.timestamp} ${trace.provider} ${trace.message}`
          );
        }
      }
    );
};
