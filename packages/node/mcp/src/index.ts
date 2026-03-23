#!/usr/bin/env node
/**
 * RunFabric MCP server — exposes doctor, plan, build, deploy, remove, invoke, logs, list, inspect, releases, generate, state, workflow for agents and IDEs.
 * Requires `runfabric` CLI on PATH (e.g. from repo: make build then bin/runfabric).
 */
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

function runfabricCmd(): string {
  return process.env.RUNFABRIC_CMD ?? "runfabric";
}

const argsSchema = z.object({
  configPath: z.string().optional().describe("Path to runfabric.yml (default: runfabric.yml)"),
  stage: z.string().optional().describe("Stage name (default: dev)"),
});

const deployArgsSchema = argsSchema.extend({
  preview: z.string().optional().describe("Preview env id, e.g. pr-123"),
  provider: z.string().optional().describe("Provider key from providerOverrides"),
});

const invokeArgsSchema = argsSchema.extend({
  function: z.string().describe("Function name to invoke"),
  payload: z.string().optional().describe("JSON payload string"),
  provider: z.string().optional(),
});

const logsArgsSchema = argsSchema.extend({
  function: z.string().optional().describe("Function name; omit to use --all"),
  all: z.boolean().optional().describe("If true, pass --all for all functions"),
  provider: z.string().optional(),
});

const generateArgsSchema = argsSchema.extend({
  args: z.array(z.string()).optional().describe("Additional args passed after `runfabric generate`"),
});

const scopedCommandArgsSchema = argsSchema.extend({
  command: z.string().describe("Subcommand after the scoped command, e.g. list, inspect, invoke"),
  args: z.array(z.string()).optional().describe("Additional args passed after the subcommand"),
});

function runRunfabric(
  subcommand: string[],
  args: { configPath?: string; stage?: string; preview?: string; provider?: string },
  extraArgs: string[] = []
): Promise<{ stdout: string; stderr: string; code: number | null }> {
  const cmdArgs: string[] = [...subcommand];
  if (args.configPath) cmdArgs.push("-c", args.configPath);
  if (args.stage) cmdArgs.push("--stage", args.stage);
  if (args.preview) cmdArgs.push("--preview", args.preview);
  if (args.provider) cmdArgs.push("--provider", args.provider);
  cmdArgs.push(...extraArgs, "--json", "--non-interactive", "--yes");

  return new Promise((resolve) => {
    const proc = spawn(runfabricCmd(), cmdArgs, {
      stdio: ["ignore", "pipe", "pipe"],
      shell: false,
    });
    let stdout = "";
    let stderr = "";
    proc.stdout?.on("data", (d: Buffer) => { stdout += d; });
    proc.stderr?.on("data", (d: Buffer) => { stderr += d; });
    proc.on("close", (code: number | null) => resolve({ stdout, stderr, code }));
    proc.on("error", (err: NodeJS.ErrnoException) => {
      const notFound = err.code === "ENOENT" || err.message?.includes("not found");
      const msg = notFound
        ? [
          "RunFabric CLI not found (runfabric not on PATH and RUNFABRIC_CMD not set).",
          "Fix: (1) Build CLI: make build, then export PATH=/path/to/repo/bin:$PATH",
          "  or (2) Set RUNFABRIC_CMD=/absolute/path/to/runfabric for the MCP server process.",
          "See @runfabric/mcp-stdio README section 'For AI agents'.",
        ].join("\n")
        : String(err.message || err);
      resolve({ stdout: "", stderr: msg, code: 1 });
    });
  });
}

type ToolArgs = { configPath?: string; stage?: string; preview?: string; provider?: string };

function resultPayload(stdout: string, stderr: string, code: number | null) {
  const out = (stdout || stderr).trim() || "No output";
  return { content: [{ type: "text" as const, text: out }], isError: code !== 0 };
}

function registerBaseTool(
  server: McpServer,
  toolName: string,
  description: string,
  subcommand: string[],
  inputSchema: z.AnyZodObject,
  extraArgsFromInput?: (args: Record<string, unknown>) => string[],
) {
  server.registerTool(
    toolName,
    {
      description,
      inputSchema,
    },
    async (rawArgs) => {
      const args = (rawArgs ?? {}) as Record<string, unknown>;
      const base: ToolArgs = {
        configPath: typeof args.configPath === "string" ? args.configPath : undefined,
        stage: typeof args.stage === "string" ? args.stage : undefined,
        preview: typeof args.preview === "string" ? args.preview : undefined,
        provider: typeof args.provider === "string" ? args.provider : undefined,
      };
      const extra = extraArgsFromInput ? extraArgsFromInput(args) : [];
      const { stdout, stderr, code } = await runRunfabric(subcommand, base, extra);
      return resultPayload(stdout, stderr, code);
    },
  );
}

export function createServer(): McpServer {
  const server = new McpServer(
    { name: "runfabric", version: "0.1.0" },
    { capabilities: { tools: { listChanged: true } } }
  );

  registerBaseTool(
    server,
    "runfabric_doctor",
    "Run runfabric doctor to validate config and credentials",
    ["doctor"],
    z.object({
      configPath: z.string().optional(),
      stage: z.string().optional(),
    })
  );

  registerBaseTool(
    server,
    "runfabric_plan",
    "Run runfabric plan to show deployment plan",
    ["plan"],
    argsSchema.extend({ provider: z.string().optional() })
  );

  registerBaseTool(
    server,
    "runfabric_build",
    "Run runfabric build to build artifacts for deploy",
    ["build"],
    argsSchema.extend({ provider: z.string().optional() })
  );

  registerBaseTool(
    server,
    "runfabric_deploy",
    "Run runfabric deploy",
    ["deploy"],
    deployArgsSchema
  );

  registerBaseTool(
    server,
    "runfabric_remove",
    "Run runfabric remove to tear down deployed resources",
    ["remove"],
    argsSchema.extend({ provider: z.string().optional() })
  );

  registerBaseTool(
    server,
    "runfabric_invoke",
    "Invoke a deployed function",
    ["invoke"],
    invokeArgsSchema,
    (args) => {
      const fn = typeof args.function === "string" ? args.function : "";
      if (!fn) return [];
      const payload = typeof args.payload === "string" ? args.payload : "";
      return ["--function", fn, ...(payload ? ["--payload", payload] : [])];
    }
  );

  registerBaseTool(
    server,
    "runfabric_logs",
    "Fetch logs for a deployed function or all functions (provider logs + optional local)",
    ["logs"],
    logsArgsSchema,
    (args) => {
      const fn = typeof args.function === "string" ? args.function : "";
      const allFlag = Boolean(args.all);
      return allFlag ? ["--all"] : fn ? ["--function", fn] : ["--all"];
    }
  );

  registerBaseTool(
    server,
    "runfabric_list",
    "List functions from runfabric.yml and deployment status from receipt",
    ["list"],
    argsSchema.extend({ provider: z.string().optional() })
  );

  registerBaseTool(
    server,
    "runfabric_inspect",
    "Show lock, journal, and receipt state for the current backend",
    ["inspect"],
    argsSchema.extend({ provider: z.string().optional() })
  );

  registerBaseTool(
    server,
    "runfabric_releases",
    "List deployment history (releases) for the stage from receipt backend",
    ["releases"],
    argsSchema.extend({ provider: z.string().optional() })
  );

  registerBaseTool(
    server,
    "runfabric_generate",
    "Run runfabric generate with optional additional arguments",
    ["generate"],
    generateArgsSchema,
    (args) => Array.isArray(args.args) ? (args.args as unknown[]).filter((v): v is string => typeof v === "string") : []
  );

  registerBaseTool(
    server,
    "runfabric_state",
    "Run runfabric state <command> with additional arguments",
    ["state"],
    scopedCommandArgsSchema,
    (args) => {
      const cmd = typeof args.command === "string" ? args.command.trim() : "";
      const rest = Array.isArray(args.args) ? (args.args as unknown[]).filter((v): v is string => typeof v === "string") : [];
      return cmd ? [cmd, ...rest] : rest;
    }
  );

  registerBaseTool(
    server,
    "runfabric_workflow",
    "Run runfabric workflow <command> with additional arguments",
    ["workflow"],
    scopedCommandArgsSchema,
    (args) => {
      const cmd = typeof args.command === "string" ? args.command.trim() : "";
      const rest = Array.isArray(args.args) ? (args.args as unknown[]).filter((v): v is string => typeof v === "string") : [];
      return cmd ? [cmd, ...rest] : rest;
    }
  );

  return server;
}

export async function startServer(): Promise<void> {
  const server = createServer();
  const transport = new StdioServerTransport();
  await server.connect(transport);
}

const isMain = process.argv[1] === fileURLToPath(import.meta.url);
if (isMain) {
  await startServer();
}
