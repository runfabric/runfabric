import { existsSync } from "node:fs";
import { readFile } from "node:fs/promises";
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
  resolveHandlerSnapshot,
  resolveTypeScriptCompiler
} from "./runtime";

const DEFAULT_MAX_REQUEST_BODY_BYTES = 1024 * 1024;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

class RequestBodyTooLargeError extends Error {
  readonly statusCode: number;

  constructor(maxBytes: number) {
    super(
      `request body exceeds ${maxBytes} bytes. Set RUNFABRIC_CALL_LOCAL_MAX_BODY_BYTES to increase the limit.`
    );
    this.statusCode = 413;
    this.name = "RequestBodyTooLargeError";
  }
}

function resolveMaxRequestBodyBytes(): number {
  const raw = process.env.RUNFABRIC_CALL_LOCAL_MAX_BODY_BYTES;
  if (!raw || raw.trim().length === 0) {
    return DEFAULT_MAX_REQUEST_BODY_BYTES;
  }

  const parsed = Number.parseInt(raw, 10);
  if (Number.isInteger(parsed) && parsed > 0) {
    return parsed;
  }

  warn(
    `invalid RUNFABRIC_CALL_LOCAL_MAX_BODY_BYTES=${JSON.stringify(raw)}; using default ${DEFAULT_MAX_REQUEST_BODY_BYTES}`
  );
  return DEFAULT_MAX_REQUEST_BODY_BYTES;
}

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

async function readRequestBody(
  request: AsyncIterable<string | Buffer>,
  maxBytes: number
): Promise<string | undefined> {
  const chunks: Buffer[] = [];
  let totalBytes = 0;
  for await (const chunk of request) {
    const chunkBuffer = typeof chunk === "string" ? Buffer.from(chunk) : chunk;
    totalBytes += chunkBuffer.byteLength;
    if (totalBytes > maxBytes) {
      throw new RequestBodyTooLargeError(maxBytes);
    }
    chunks.push(chunkBuffer);
  }
  if (chunks.length === 0) {
    return undefined;
  }
  return Buffer.concat(chunks).toString("utf8");
}

function mergeTemplateObject(
  template: Record<string, unknown>,
  requestEvent: Record<string, unknown>
): Record<string, unknown> {
  const merged: Record<string, unknown> = { ...template };
  for (const [key, value] of Object.entries(requestEvent)) {
    const templateValue = merged[key];
    if (isRecord(templateValue) && isRecord(value)) {
      merged[key] = mergeTemplateObject(templateValue, value);
      continue;
    }
    merged[key] = value;
  }
  return merged;
}

export function mergeServeEventTemplate(templateEvent: unknown, requestEvent: unknown): unknown {
  if (!isRecord(templateEvent) || !isRecord(requestEvent)) {
    return requestEvent;
  }
  return mergeTemplateObject(templateEvent, requestEvent);
}

async function loadServeEventTemplate(
  projectDir: string,
  eventPath: string | undefined
): Promise<Record<string, unknown> | undefined> {
  if (!eventPath) {
    return undefined;
  }

  const raw = await readFile(resolve(projectDir, eventPath), "utf8");
  const parsed = JSON.parse(raw) as unknown;
  if (!isRecord(parsed)) {
    throw new Error("--event must point to a JSON object when used with --serve");
  }
  return parsed;
}

function writeServerError(response: ServerResponse, serverError: unknown): void {
  const message = serverError instanceof Error ? serverError.message : String(serverError);
  const statusCode =
    serverError && typeof serverError === "object" && "statusCode" in serverError
      ? Number((serverError as { statusCode?: unknown }).statusCode) || 500
      : 500;
  response.statusCode = statusCode;
  response.setHeader("content-type", "application/json");
  response.end(JSON.stringify({ error: message }));
}

type LoadedHandler = NonNullable<Awaited<ReturnType<typeof resolveExecutionContext>>["handler"]>;

export function createHandlerResolver<T>(options: {
  watchMode: boolean;
  initialHandler?: T;
  loadFreshHandler: () => Promise<T>;
  readWatchVersion: () => Promise<string>;
}): () => Promise<T> {
  if (!options.watchMode) {
    if (options.initialHandler === undefined) {
      throw new Error("handler is not loaded");
    }
    return async () => options.initialHandler as T;
  }

  let cachedVersion: string | undefined;
  let cachedHandler = options.initialHandler;
  let pendingLoad: Promise<T> | undefined;

  return async () => {
    if (pendingLoad) {
      return pendingLoad;
    }

    pendingLoad = (async () => {
      const nextVersion = await options.readWatchVersion();
      if (cachedHandler === undefined || nextVersion !== cachedVersion) {
        cachedHandler = await options.loadFreshHandler();
        cachedVersion = nextVersion;
      }
      return cachedHandler;
    })();

    try {
      return await pendingLoad;
    } finally {
      pendingLoad = undefined;
    }
  };
}

async function invokeServeRequest(
  projectDir: string,
  provider: string,
  resolveHandler: () => Promise<LoadedHandler>,
  host: string,
  port: number,
  maxRequestBodyBytes: number,
  extraHeaders: Record<string, string>,
  eventTemplate: Record<string, unknown> | undefined,
  request: IncomingMessage
) {
  const requestUrl = new URL(request.url || "/", `http://${host}:${port}`);
  const parsed: ParsedCallOptions = {
    provider,
    method: (request.method || "GET").toUpperCase(),
    path: requestUrl.pathname,
    query: requestUrl.search.startsWith("?") ? requestUrl.search.slice(1) : requestUrl.search,
    body: await readRequestBody(request, maxRequestBodyBytes),
    headers: {
      ...normalizeNodeHeaders(request.headers),
      ...extraHeaders
    }
  };
  const generatedEvent = createEventFromOptions(parsed);
  const event = eventTemplate
    ? mergeServeEventTemplate(eventTemplate, generatedEvent)
    : generatedEvent;
  const requestHandler = await resolveHandler();
  return invokeWithProvider(provider, requestHandler, event);
}

function createServeServer(params: {
  projectDir: string;
  provider: string;
  resolveHandler: () => Promise<LoadedHandler>;
  host: string;
  port: number;
  maxRequestBodyBytes: number;
  extraHeaders: Record<string, string>;
  eventTemplate: Record<string, unknown> | undefined;
}) {
  return createServer(async (request, response) => {
    try {
      const invokeResult = await invokeServeRequest(
        params.projectDir,
        params.provider,
        params.resolveHandler,
        params.host,
        params.port,
        params.maxRequestBodyBytes,
        params.extraHeaders,
        params.eventTemplate,
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
  const { provider, entry, handler } = await resolveExecutionContext(projectDir, options);
  const host = options.host || "127.0.0.1";
  const port = options.port ?? 8787;
  const maxRequestBodyBytes = resolveMaxRequestBodyBytes();
  const watchMode = Boolean(options.watch);
  const extraHeaders = parseHeaders(options.header || []);
  const eventTemplate = await loadServeEventTemplate(projectDir, options.event);
  const resolveHandler = createHandlerResolver<LoadedHandler>({
    watchMode,
    initialHandler: handler || undefined,
    loadFreshHandler: async () => {
      const loaded = await loadHandler(projectDir, entry, true);
      if (!loaded) {
        throw new Error("handler is not loaded");
      }
      return loaded;
    },
    readWatchVersion: async () => {
      const snapshot = await resolveHandlerSnapshot(projectDir, entry);
      return snapshot.version;
    }
  });
  const server = createServeServer({
    projectDir,
    provider,
    resolveHandler,
    host,
    port,
    maxRequestBodyBytes,
    extraHeaders,
    eventTemplate
  });
  await listenServeServer(server, host, port);

  const serverAddress = server.address();
  const resolvedPort = serverAddress && typeof serverAddress === "object" ? serverAddress.port : port;
  const tscWatch = watchMode && /\.(ts|tsx)$/i.test(entry) ? startTypeScriptWatch(projectDir) : undefined;
  logServeStartup(host, resolvedPort, provider, entry, watchMode);
  await waitForServeShutdown(server, tscWatch);
}
