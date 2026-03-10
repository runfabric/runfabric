#!/usr/bin/env node
import { Command } from "commander";
import { registerBuildCommand } from "./commands/build";
import { registerComposeCommand } from "./commands/compose";
import { registerDeployCommand } from "./commands/deploy";
import { registerDoctorCommand } from "./commands/doctor";
import { registerInitCommand } from "./commands/init";
import { registerInvokeCommand } from "./commands/invoke";
import { registerLogsCommand } from "./commands/logs";
import { registerPackageCommand } from "./commands/package";
import { registerPlanCommand } from "./commands/plan";
import { registerPrimitivesCommand } from "./commands/primitives";
import { registerProvidersCommand } from "./commands/providers";
import { registerRemoveCommand } from "./commands/remove";

const program = new Command();

program
  .name("runfabric")
  .description("Multi-provider serverless deployment framework");

registerInitCommand(program);
registerDoctorCommand(program);
registerPlanCommand(program);
registerBuildCommand(program);
registerPackageCommand(program);
registerDeployCommand(program);
registerRemoveCommand(program);
registerInvokeCommand(program);
registerLogsCommand(program);
registerProvidersCommand(program);
registerPrimitivesCommand(program);
registerComposeCommand(program);

void program.parseAsync(process.argv).catch((err: unknown) => {
  const message = err instanceof Error ? err.message : String(err);
  console.error(`runfabric error: ${message}`);
  process.exitCode = 1;
});
