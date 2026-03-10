import type { CommandRegistrar } from "../types/cli";
import { createProviderRegistry } from "../providers/registry";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info } from "../utils/logger";

export const registerLogsCommand: CommandRegistrar = (program) => {
  program
    .command("logs")
    .description("Fetch provider logs for a function")
    .option("-p, --provider <name>", "Provider name", "aws-lambda")
    .action(async (options: { provider: string }) => {
      const projectDir = await resolveProjectDir();
      const providers = createProviderRegistry(projectDir);
      const provider = providers[options.provider];
      if (!provider) {
        error(`unknown provider: ${options.provider}`);
        process.exitCode = 1;
        return;
      }

      if (!provider.logs) {
        info(`${provider.name}: logs are not implemented`);
        return;
      }

      const result = await provider.logs({ provider: provider.name });
      for (const line of result.lines) {
        info(line);
      }
    });
};
