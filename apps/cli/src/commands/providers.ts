import type { CommandRegistrar } from "../types/cli";
import { resolve } from "node:path";
import { KNOWN_PROVIDER_IDS, createProviderRegistry, getProviderPackageName } from "../providers/registry";
import { info, warn } from "../utils/logger";

export const registerProvidersCommand: CommandRegistrar = (program) => {
  program
    .command("providers")
    .description("List available provider adapters")
    .action(() => {
      const projectDir = resolve(process.cwd());
      const providers = createProviderRegistry(projectDir, KNOWN_PROVIDER_IDS);
      for (const providerId of KNOWN_PROVIDER_IDS) {
        const provider = providers[providerId];
        if (!provider) {
          const packageName = getProviderPackageName(providerId);
          warn(
            packageName
              ? `${providerId} | not installed (add ${packageName})`
              : `${providerId} | not installed`
          );
          continue;
        }
        const capabilities = provider.getCapabilities();
        const runtimes = capabilities.supportedRuntimes.join(",");
        info(
          `${provider.name} | http=${capabilities.http} cron=${capabilities.cron} queue=${capabilities.queue} edge=${capabilities.edgeRuntime} runtimes=${runtimes}`
        );
      }
    });
};
