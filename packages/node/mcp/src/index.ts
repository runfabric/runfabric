#!/usr/bin/env node
/**
 * RunFabric MCP server — exposes doctor, plan, build, deploy, remove, invoke, logs, list, inspect, releases for agents and IDEs.
 * Requires `runfabric` CLI on PATH (e.g. from repo: make build then bin/runfabric).
 */
import { spawn } from "node:child_process";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

const RUNFABRIC_CMD = process.env.RUNFABRIC_CMD ?? "runfabric";

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
    const proc = spawn(RUNFABRIC_CMD, cmdArgs, {
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

const server = new McpServer(
  { name: "runfabric", version: "0.1.0" },
  { capabilities: { tools: { listChanged: true } } }
);

server.registerTool(
  "runfabric_doctor",
  {
    description: "Run runfabric doctor to validate config and credentials",
    inputSchema: z.object({
      configPath: z.string().optional(),
      stage: z.string().optional(),
    }),
  },
  async (args) => {
    const { configPath, stage } = args ?? {};
    const { stdout, stderr, code } = await runRunfabric(["doctor"], { configPath, stage });
    const out = (stdout || stderr).trim() || "No output";
    return { content: [{ type: "text", text: out }], isError: code !== 0 };
  }
);

server.registerTool(
  "runfabric_plan",
  {
    description: "Run runfabric plan to show deployment plan",
    inputSchema: argsSchema.extend({ provider: z.string().optional() }),
  },
  async (args) => {
    const { configPath, stage, provider } = args ?? {};
    const { stdout, stderr, code } = await runRunfabric(["plan"], { configPath, stage, provider });
    const out = (stdout || stderr).trim() || "No output";
    return { content: [{ type: "text", text: out }], isError: code !== 0 };
  }
);

server.registerTool(
  "runfabric_build",
  {
    description: "Run runfabric build to build artifacts for deploy",
    inputSchema: argsSchema.extend({ provider: z.string().optional() }),
  },
  async (args) => {
    const { configPath, stage, provider } = args ?? {};
    const { stdout, stderr, code } = await runRunfabric(["build"], { configPath, stage, provider });
    const out = (stdout || stderr).trim() || "No output";
    return { content: [{ type: "text", text: out }], isError: code !== 0 };
  }
);

server.registerTool(
  "runfabric_deploy",
  {
    description: "Run runfabric deploy",
    inputSchema: deployArgsSchema,
  },
  async (args) => {
    const { configPath, stage, preview, provider } = args ?? {};
    const { stdout, stderr, code } = await runRunfabric(["deploy"], {
      configPath,
      stage,
      preview,
      provider,
    });
    const out = (stdout || stderr).trim() || "No output";
    return { content: [{ type: "text", text: out }], isError: code !== 0 };
  }
);

server.registerTool(
  "runfabric_remove",
  {
    description: "Run runfabric remove to tear down deployed resources",
    inputSchema: argsSchema.extend({ provider: z.string().optional() }),
  },
  async (args) => {
    const { configPath, stage, provider } = args ?? {};
    const { stdout, stderr, code } = await runRunfabric(["remove"], { configPath, stage, provider });
    const out = (stdout || stderr).trim() || "No output";
    return { content: [{ type: "text", text: out }], isError: code !== 0 };
  }
);

server.registerTool(
  "runfabric_invoke",
  {
    description: "Invoke a deployed function",
    inputSchema: invokeArgsSchema,
  },
  async (args) => {
    const { configPath, stage, function: fn, payload, provider } = args ?? {};
    if (!fn) return { content: [{ type: "text", text: "Missing required argument: function" }], isError: true };
    const extraArgs = ["--function", fn, ...(payload ? ["--payload", payload] : [])];
    const { stdout, stderr, code } = await runRunfabric(
      ["invoke"],
      { configPath, stage, provider },
      extraArgs
    );
    const out = (stdout || stderr).trim() || "No output";
    return { content: [{ type: "text", text: out }], isError: code !== 0 };
  }
);

server.registerTool(
  "runfabric_logs",
  {
    description: "Fetch logs for a deployed function or all functions (provider logs + optional local)",
    inputSchema: logsArgsSchema,
  },
  async (args) => {
    const { configPath, stage, function: fn, all: allFlag, provider } = args ?? {};
    const extraArgs = allFlag ? ["--all"] : fn ? ["--function", fn] : ["--all"];
    const { stdout, stderr, code } = await runRunfabric(
      ["logs"],
      { configPath, stage, provider },
      extraArgs
    );
    const out = (stdout || stderr).trim() || "No output";
    return { content: [{ type: "text", text: out }], isError: code !== 0 };
  }
);

server.registerTool(
  "runfabric_list",
  {
    description: "List functions from runfabric.yml and deployment status from receipt",
    inputSchema: argsSchema.extend({ provider: z.string().optional() }),
  },
  async (args) => {
    const { configPath, stage, provider } = args ?? {};
    const { stdout, stderr, code } = await runRunfabric(["list"], { configPath, stage, provider });
    const out = (stdout || stderr).trim() || "No output";
    return { content: [{ type: "text", text: out }], isError: code !== 0 };
  }
);

server.registerTool(
  "runfabric_inspect",
  {
    description: "Show lock, journal, and receipt state for the current backend",
    inputSchema: argsSchema.extend({ provider: z.string().optional() }),
  },
  async (args) => {
    const { configPath, stage, provider } = args ?? {};
    const { stdout, stderr, code } = await runRunfabric(["inspect"], { configPath, stage, provider });
    const out = (stdout || stderr).trim() || "No output";
    return { content: [{ type: "text", text: out }], isError: code !== 0 };
  }
);

server.registerTool(
  "runfabric_releases",
  {
    description: "List deployment history (releases) for the stage from receipt backend",
    inputSchema: argsSchema.extend({ provider: z.string().optional() }),
  },
  async (args) => {
    const { configPath, stage, provider } = args ?? {};
    const { stdout, stderr, code } = await runRunfabric(["releases"], { configPath, stage, provider });
    const out = (stdout || stderr).trim() || "No output";
    return { content: [{ type: "text", text: out }], isError: code !== 0 };
  }
);

const transport = new StdioServerTransport();
await server.connect(transport);
