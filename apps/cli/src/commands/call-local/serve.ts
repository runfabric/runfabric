import { existsSync } from "node:fs";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { resolve } from "node:path";
import { spawn, type ChildProcess } from "node:child_process";
import { info, warn } from "../../utils/logger";
import type { LocalCallOptions, ParsedCallOptions } from "./runtime";
import {
  createEventFromOptions,
  invokeWithProvider,
  loadHandler,
  normalizeNodeHeaders,
  parseHeaders,
  resolveExecutionContext,
  resolveTypeScriptCompiler
} from "./runtime";

function startTypeScriptWatch(projectDir: string): ChildProcess | undefined {
  const tsconfigPath = resolve(projectDir, "tsconfig.json");
  if (!existsSync(tsconfigPath)) {
    warn("watch mode requested for TypeScript entry but tsconfig.json was not found; skipping tsc --watch");
    return undefined;
  }

  info("watch mode: starting TypeScript compiler (tsc --watch -p tsconfig.json)");
  const child = spawn(resolveTypeScriptCompiler(projectDir), ["--watch", "-p", "tsconfig.json"], {
    cwd: projectDir,
    stdio: "inherit",
    env: process.env,
    detached: process.platform !== "win32"
  });
  child.on("error", (watchError) => {
    const message = watchError instanceof Error ? watchError.message : String(watchError);
    warn(`failed to start tsc --watch: ${message}`);
  });
  return child;
}

async function stopChildProcess(child: ChildProcess, timeoutMs = 1500): Promise<void> {
  if (child.killed || child.exitCode !== null) {
    return;
  }

  const terminate = (signal: NodeJS.Signals): void => {
    if (process.platform !== "win32" && typeof child.pid === "number") {
      try {
        process.kill(-child.pid, signal);
        return;
      } catch {
        // fall back to direct kill
      }
    }
    child.kill(signal);
  };

  const exited = new Promise<void>((resolvePromise) => child.once("exit", () => resolvePromise()));
  terminate("SIGTERM");
  await Promise.race([
    exited,
    new Promise<void>((resolvePromise) => {
      setTimeout(() => {
        if (!child.killed && child.exitCode === null) {
          terminate("SIGKILL");
        }
        resolvePromise();
      }, timeoutMs);
    })
  ]);
}

async function readRequestBody(request: AsyncIterable<string | Buffer>): Promise<string | undefined> {
  const chunks: Buffer[] = [];
  for await (const chunk of request) {
    chunks.push(typeof chunk === "string" ? Buffer.from(chunk) : chunk);
  }
  if (chunks.length === 0) {
    return undefined;
  }
  return Buffer.concat(chunks).toString("utf8");
}

function writeServerError(response: ServerResponse, serverError: unknown): void {
  const message = serverError instanceof Error ? serverError.message : String(serverError);
  response.statusCode = 500;
  response.setHeader("content-type", "application/json");
  response.end(JSON.stringify({ error: message }));
}

async function invokeServeRequest(
  projectDir: string,
  provider: string,
  entry: string,
  watchMode: boolean,
  handler: Awaited<ReturnType<typeof resolveExecutionContext>>["handler"],
  host: string,
  port: number,
  extraHeaders: Record<string, string>,
  request: IncomingMessage
) {
  const requestUrl = new URL(request.url || "/", `http://${host}:${port}`);
  const parsed: ParsedCallOptions = {
    provider,
    method: (request.method || "GET").toUpperCase(),
    path: requestUrl.pathname,
    query: requestUrl.search.startsWith("?") ? requestUrl.search.slice(1) : requestUrl.search,
    body: await readRequestBody(request),
    headers: {
      ...normalizeNodeHeaders(request.headers),
      ...extraHeaders
    }
  };
  const event = createEventFromOptions(parsed);
  const requestHandler = watchMode ? await loadHandler(projectDir, entry, true) : handler;
  if (!requestHandler) {
    throw new Error("handler is not loaded");
  }
  return invokeWithProvider(provider, requestHandler, event);
}

function createServeServer(params: {
  projectDir: string;
  provider: string;
  entry: string;
  watchMode: boolean;
  handler: Awaited<ReturnType<typeof resolveExecutionContext>>["handler"];
  host: string;
  port: number;
  extraHeaders: Record<string, string>;
}) {
  return createServer(async (request, response) => {
    try {
      const invokeResult = await invokeServeRequest(
        params.projectDir,
        params.provider,
        params.entry,
        params.watchMode,
        params.handler,
        params.host,
        params.port,
        params.extraHeaders,
        request
      );
      response.statusCode = invokeResult.statusCode;
      for (const [key, value] of Object.entries(invokeResult.headers)) {
        response.setHeader(key, value);
      }
      response.end(invokeResult.body);
    } catch (serverError) {
      writeServerError(response, serverError);
    }
  });
}

async function listenServeServer(
  server: ReturnType<typeof createServer>,
  host: string,
  port: number
): Promise<void> {
  await new Promise<void>((resolvePromise, rejectPromise) => {
    server.once("error", rejectPromise);
    server.listen(port, host, () => resolvePromise());
  });
}

function logServeStartup(host: string, port: number, provider: string, entry: string, watchMode: boolean): void {
  info(`local call server listening on http://${host}:${port}`);
  info(`provider: ${provider}`);
  info(`entry: ${entry}`);
  if (watchMode) {
    info("watch mode enabled");
  }
  if (process.stdin.isTTY) {
    info("press Ctrl+C or type 'exit' and Enter to stop");
  }
}

async function waitForServeShutdown(
  server: ReturnType<typeof createServer>,
  tscWatch: ChildProcess | undefined
): Promise<void> {
  await new Promise<void>((resolvePromise) => {
    let shuttingDown = false;
    let stdinListenerAttached = false;
    const onStdinData = (chunk: string | Buffer): void => {
      const text = chunk.toString().trim().toLowerCase();
      if (text === "q" || text === "quit" || text === "exit") {
        void shutdown();
      }
    };

    const shutdown = async (): Promise<void> => {
      if (shuttingDown) {
        return;
      }
      shuttingDown = true;
      info("shutting down local call server");
      process.off("SIGINT", shutdown);
      process.off("SIGTERM", shutdown);
      process.off("SIGQUIT", shutdown);
      if (stdinListenerAttached) {
        process.stdin.off("data", onStdinData);
        if (typeof process.stdin.pause === "function") {
          process.stdin.pause();
        }
      }
      server.close(async () => {
        if (tscWatch) {
          await stopChildProcess(tscWatch);
        }
        resolvePromise();
      });
    };

    process.on("SIGINT", shutdown);
    process.on("SIGTERM", shutdown);
    process.on("SIGQUIT", shutdown);
    if (process.stdin.isTTY) {
      process.stdin.on("data", onStdinData);
      process.stdin.resume();
      stdinListenerAttached = true;
    }
  });
}

export async function serveLocalCalls(projectDir: string, options: LocalCallOptions): Promise<void> {
  if (options.event) {
    throw new Error("--event is not supported with --serve");
  }

  const { provider, entry, handler } = await resolveExecutionContext(projectDir, options);
  const host = options.host || "127.0.0.1";
  const port = options.port ?? 8787;
  const watchMode = Boolean(options.watch);
  const extraHeaders = parseHeaders(options.header || []);
  const server = createServeServer({ projectDir, provider, entry, watchMode, handler, host, port, extraHeaders });
  await listenServeServer(server, host, port);

  const serverAddress = server.address();
  const resolvedPort = serverAddress && typeof serverAddress === "object" ? serverAddress.port : port;
  const tscWatch = watchMode && entry.endsWith(".ts") ? startTypeScriptWatch(projectDir) : undefined;
  logServeStartup(host, resolvedPort, provider, entry, watchMode);
  await waitForServeShutdown(server, tscWatch);
}
