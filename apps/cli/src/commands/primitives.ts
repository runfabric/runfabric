import type { CommandRegistrar } from "../types/cli";
import { PLATFORM_PRIMITIVES } from "@runfabric/core";
import { primitiveProfiles } from "@runfabric/planner";
import { info } from "../utils/logger";

export const registerPrimitivesCommand: CommandRegistrar = (program) => {
  program
    .command("primitives")
    .description("Show provider primitive compatibility profiles")
    .action(() => {
      for (const [provider, profile] of Object.entries(primitiveProfiles)) {
        const supported = PLATFORM_PRIMITIVES.filter((primitive) => profile[primitive]);
        info(`${provider}: ${supported.join(", ")}`);
      }
    });
};
