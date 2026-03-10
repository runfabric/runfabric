import type { CommandRegistrar } from "../types/cli";
import { resolve } from "node:path";
import { createProviderRegistry } from "../providers/registry";
import { info } from "../utils/logger";

export const registerProvidersCommand: CommandRegistrar = (program) => {
  program
    .command("providers")
    .description("List available provider adapters")
    .action(() => {
      const providers = createProviderRegistry(resolve(process.cwd()));
      for (const provider of Object.values(providers)) {
        const capabilities = provider.getCapabilities();
        info(
          `${provider.name} | http=${capabilities.http} cron=${capabilities.cron} queue=${capabilities.queue} edge=${capabilities.edgeRuntime}`
        );
      }
    });
};
