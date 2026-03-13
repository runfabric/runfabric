#!/usr/bin/env node
import { Command } from "commander";
import { registerBuildCommand } from "./commands/build";
import { registerCallLocalCommand } from "./commands/call-local";
import { registerComposeCommand } from "./commands/compose";
import { registerDeployCommand } from "./commands/deploy";
import { registerDoctorCommand } from "./commands/doctor";
import { registerDocsCommand } from "./commands/docs";
import { registerInitCommand } from "./commands/init";
import { registerInvokeCommand } from "./commands/invoke";
import { registerLogsCommand } from "./commands/logs";
import { registerMetricsCommand } from "./commands/metrics";
import { registerMigrateCommand } from "./commands/migrate";
import { registerPackageCommand } from "./commands/package";
import { registerPlanCommand } from "./commands/plan";
import { registerPrimitivesCommand } from "./commands/primitives";
import { registerProvidersCommand } from "./commands/providers";
import { registerRemoveCommand } from "./commands/remove";
import { registerStateCommand } from "./commands/state";
import { registerTracesCommand } from "./commands/traces";
import { registerDevCommand } from "./commands/dev";

const program = new Command();
const packageJson = require("../package.json") as { version?: string };
const cliVersion = typeof packageJson.version === "string" ? packageJson.version : "0.0.0";

program
  .name("runfabric")
  .description("Multi-provider serverless deployment framework")
  .version(cliVersion);

program
  .command("version")
  .description("Print CLI version")
  .action(() => {
    process.stdout.write(`${cliVersion}\n`);
  });

registerInitCommand(program);
registerDocsCommand(program);
registerDoctorCommand(program);
registerPlanCommand(program);
registerBuildCommand(program);
registerPackageCommand(program);
registerDeployCommand(program);
registerRemoveCommand(program);
registerInvokeCommand(program);
registerCallLocalCommand(program);
registerLogsCommand(program);
registerTracesCommand(program);
registerMetricsCommand(program);
registerMigrateCommand(program);
registerProvidersCommand(program);
registerPrimitivesCommand(program);
registerComposeCommand(program);
registerStateCommand(program);
registerDevCommand(program);

void program.parseAsync(process.argv).catch((err: unknown) => {
  const message = err instanceof Error ? err.message : String(err);
  console.error(`runfabric error: ${message}`);
  process.exitCode = 1;
});
