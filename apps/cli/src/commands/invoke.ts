import type { CommandRegistrar } from "../types/cli";
import { createProviderRegistry } from "../providers/registry";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { error } from "../utils/logger";

export const registerInvokeCommand: CommandRegistrar = (program) => {
  program
    .command("invoke")
    .description("Invoke a deployed function")
    .option("-p, --provider <name>", "Provider name", "aws-lambda")
    .option("--payload <json>", "JSON payload string")
    .action(async (options: { provider: string; payload?: string }) => {
      const projectDir = await resolveProjectDir();
      const providers = createProviderRegistry(projectDir);
      const provider = providers[options.provider];
      if (!provider) {
        error(`unknown provider: ${options.provider}`);
        process.exitCode = 1;
        return;
      }

      if (!provider.invoke) {
        error(`${provider.name}: invoke is not supported by this adapter`);
        process.exitCode = 1;
        return;
      }

      const result = await provider.invoke({
        provider: provider.name,
        payload: options.payload
      });
      printJson(result);
    });
};
