import type { CommandRegistrar } from "../types/cli";
import { error } from "../utils/logger";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { serveLocalCalls } from "./call-local/serve";
import { executeLocalCall, type LocalCallOptions } from "./call-local/runtime";

function collectHeader(value: string, previous: string[]): string[] {
  return [...previous, value];
}

function parsePort(value: string): number {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isInteger(parsed) || parsed < 0 || parsed > 65535) {
    throw new Error(`invalid port: ${value}`);
  }
  return parsed;
}

export { executeLocalCall };
export type { LocalCallOptions };

export const registerCallLocalCommand: CommandRegistrar = (program) => {
  program
    .command("call-local")
    .description("Invoke local handler with provider-mimic request payload")
    .option("-c, --config <path>", "Path to runfabric config")
    .option("-p, --provider <name>", "Provider to emulate")
    .option("--method <method>", "HTTP method", "GET")
    .option("--path <path>", "Request path", "/hello")
    .option("--query <query>", "Query string without leading ?", "")
    .option("--body <body>", "Request body")
    .option("--event <path>", "Path to a full event JSON payload")
    .option("--header <key:value>", "Header pair (repeatable)", collectHeader, [])
    .option("--entry <path>", "Handler entry override")
    .option("--serve", "Start local HTTP server mode")
    .option("--watch", "Reload handler when compiled module changes (for TS entry also runs tsc --watch)")
    .option("--host <host>", "Host for local HTTP server", "127.0.0.1")
    .option("--port <number>", "Port for local HTTP server", parsePort, 8787)
    .action(async (options: LocalCallOptions) => {
      try {
        const projectDir = await resolveProjectDir(process.cwd(), options.config);
        if (options.serve) {
          await serveLocalCalls(projectDir, options);
          return;
        }
        const result = await executeLocalCall(projectDir, options);
        printJson(result);
      } catch (callError) {
        const message = callError instanceof Error ? callError.message : String(callError);
        error(message);
        process.exitCode = 1;
      }
    });
};
