import { access, readFile } from "node:fs/promises";
import { constants, existsSync } from "node:fs";
import { createServer } from "node:http";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { spawn, type ChildProcess } from "node:child_process";
import type { UniversalHandler } from "@runfabric/core";
import { PROVIDER_IDS } from "@runfabric/core";
import {
  createAlibabaHttpAdapter,
  createAwsAdapter,
  createAzureHttpAdapter,
  createCloudflareWorkerAdapter,
  createDigitalOceanHttpAdapter,
  createFlyHttpAdapter,
  createGcpHttpAdapter,
  createIbmOpenWhiskAdapter,
  createNetlifyHttpAdapter,
  createVercelHttpAdapter
} from "@runfabric/runtime-node";
import type { CommandRegistrar } from "../types/cli";
import { loadPlanningContext } from "../utils/load-config";
import { printJson } from "../utils/output";
import { resolveProjectDir } from "../utils/resolve-project";
import { error, info, warn } from "../utils/logger";

interface LocalCallOptions {
  config?: string;
  provider?: string;
  method: string;
  path: string;
  query?: string;
  body?: string;
  event?: string;
  header: string[];
  entry?: string;
  serve?: boolean;
  watch?: boolean;
  host: string;
  port: number;
}

interface ParsedCallOptions {
  provider: string;
  method: string;
  path: string;
  query: string;
  body?: string;
  headers: Record<string, string>;
  eventPath?: string;
}

interface LocalInvokeResponse {
  statusCode: number;
  headers: Record<string, string>;
  body: string;
}

interface LocalExecutionContext {
  provider: string;
  entry: string;
  handler?: UniversalHandler;
}

function parseHeaders(headerPairs: string[]): Record<string, string> {
  const headers: Record<string, string> = {};
  for (const pair of headerPairs) {
    const separatorIndex = pair.indexOf(":");
    if (separatorIndex <= 0) {
      continue;
    }
    const key = pair.slice(0, separatorIndex).trim();
    const value = pair.slice(separatorIndex + 1).trim();
    if (!key) {
      continue;
    }
    headers[key] = value;
  }
  return headers;
}

function parseQuery(queryString: string): Record<string, string | string[]> {
  const params = new URLSearchParams(queryString);
  const query: Record<string, string | string[]> = {};
  for (const [key, value] of params.entries()) {
    if (!(key in query)) {
      query[key] = value;
      continue;
    }
    const current = query[key];
    if (Array.isArray(current)) {
      current.push(value);
      continue;
    }
    query[key] = [current, value];
  }
  return query;
}

function createEventFromOptions(options: ParsedCallOptions): unknown {
  const query = parseQuery(options.query);
  const search = options.query ? `?${options.query}` : "";

  switch (options.provider) {
    case "aws-lambda":
      return {
        version: "2.0",
        routeKey: "$default",
        rawPath: options.path,
        rawQueryString: options.query,
        headers: options.headers,
        queryStringParameters: query,
        requestContext: { http: { method: options.method, path: options.path } },
        body: options.body,
        isBase64Encoded: false
      };
    case "gcp-functions":
    case "azure-functions":
    case "vercel":
    case "fly-machines":
      return {
        method: options.method,
        path: options.path,
        url: `${options.path}${search}`,
        headers: options.headers,
        query,
        body: options.body
      };
    case "netlify":
      return {
        httpMethod: options.method,
        path: options.path,
        headers: options.headers,
        queryStringParameters: query,
        body: options.body
      };
    case "digitalocean-functions":
    case "ibm-openwhisk":
      return {
        __ow_method: options.method,
        __ow_path: options.path,
        __ow_headers: options.headers,
        __ow_query: query,
        __ow_body: options.body
      };
    case "alibaba-fc":
      return {
        httpMethod: options.method,
        path: options.path,
        headers: options.headers,
        queryParameters: query,
        body: options.body
      };
    case "cloudflare-workers":
      return {
        method: options.method,
        url: `https://local.runfabric.dev${options.path}${search}`,
        headers: options.headers,
        body: options.body
      };
    default:
      throw new Error(`unsupported provider for local call: ${options.provider}`);
  }
}

function normalizeEntryPath(entry: string): string {
  return entry.replace(/\\/g, "/").replace(/^\.\//, "");
}

function shouldAllowTypeScriptFallback(): boolean {
  const execArgv = process.execArgv.join(" ").toLowerCase();
  return (
    execArgv.includes("tsx") ||
    execArgv.includes("ts-node") ||
    execArgv.includes("register-loader") ||
    Boolean(process.env.TS_NODE_PROJECT) ||
    Boolean(process.env.TSX_TSCONFIG_PATH)
  );
}

function withExtensions(basePath: string): string[] {
  return [`${basePath}.js`, `${basePath}.mjs`, `${basePath}.cjs`];
}

function resolveHandlerCandidates(projectDir: string, entry: string): string[] {
  const normalizedEntry = normalizeEntryPath(entry);
  const candidates: string[] = [];
  const allowTypeScriptFallback = shouldAllowTypeScriptFallback();

  if (normalizedEntry.endsWith(".ts")) {
    const withoutExtension = normalizedEntry.slice(0, -3);

    // Prefer generated JavaScript artifacts first.
    candidates.push(
      ...withExtensions(resolve(projectDir, withoutExtension)),
      ...withExtensions(resolve(projectDir, "dist", withoutExtension))
    );

    if (normalizedEntry.startsWith("src/")) {
      const withoutSrcPrefix = withoutExtension.slice(4);
      candidates.push(...withExtensions(resolve(projectDir, "dist", withoutSrcPrefix)));
    }

    if (allowTypeScriptFallback) {
      candidates.push(resolve(projectDir, normalizedEntry));
    }
  } else {
    candidates.push(resolve(projectDir, normalizedEntry));

    if (
      allowTypeScriptFallback &&
      (normalizedEntry.endsWith(".js") || normalizedEntry.endsWith(".mjs") || normalizedEntry.endsWith(".cjs")) &&
      normalizedEntry.startsWith("dist/")
    ) {
      const tsSource = normalizedEntry
        .slice(5)
        .replace(/\.(js|mjs|cjs)$/i, ".ts");
      candidates.push(resolve(projectDir, "src", tsSource));
    }
  }

  return [...new Set(candidates)];
}

async function resolveHandlerPath(projectDir: string, entry: string): Promise<string> {
  const candidates = resolveHandlerCandidates(projectDir, entry);
  for (const candidate of candidates) {
    try {
      await access(candidate, constants.F_OK);
      return candidate;
    } catch {
      // keep searching
    }
  }
  throw new Error(
    `handler module not found. searched: ${candidates.join(
      ", "
    )}. For TypeScript projects run your build (for example: tsc -p tsconfig.json) before call-local in published CLI usage.`
  );
}

async function hasBuiltHandlerArtifact(projectDir: string, entry: string): Promise<boolean> {
  const candidates = resolveHandlerCandidates(projectDir, entry).filter((candidate) =>
    /\.(js|mjs|cjs)$/i.test(candidate)
  );
  for (const candidate of candidates) {
    try {
      await access(candidate, constants.F_OK);
      return true;
    } catch {
      // continue
    }
  }
  return false;
}

function resolveTypeScriptCompiler(projectDir: string): string {
  const localTsc = resolve(
    projectDir,
    "node_modules",
    ".bin",
    process.platform === "win32" ? "tsc.cmd" : "tsc"
  );
  return existsSync(localTsc) ? localTsc : "tsc";
}

async function runTypeScriptBuild(projectDir: string): Promise<void> {
  const tsconfigPath = resolve(projectDir, "tsconfig.json");
  if (!existsSync(tsconfigPath)) {
    throw new Error(
      "TypeScript entry detected but tsconfig.json was not found. Add tsconfig.json or set entry to a built JavaScript file."
    );
  }

  const compiler = resolveTypeScriptCompiler(projectDir);
  info("no built handler artifact found; running TypeScript build (tsc -p tsconfig.json)");
  await new Promise<void>((resolvePromise, rejectPromise) => {
    const child = spawn(compiler, ["-p", "tsconfig.json"], {
      cwd: projectDir,
      stdio: "inherit",
      env: process.env
    });
    child.on("error", (buildError) => {
      if ((buildError as NodeJS.ErrnoException).code === "ENOENT") {
        rejectPromise(
          new Error(
            "TypeScript compiler was not found. Install TypeScript in this project (for example: npm install -D typescript)."
          )
        );
        return;
      }
      rejectPromise(buildError);
    });
    child.on("close", (code) => {
      if (code === 0) {
        resolvePromise();
        return;
      }
      rejectPromise(new Error(`TypeScript build failed with exit code ${code ?? 1}`));
    });
  });
}

async function ensureTypeScriptArtifacts(projectDir: string, entry: string): Promise<void> {
  const normalizedEntry = normalizeEntryPath(entry);
  if (!normalizedEntry.endsWith(".ts")) {
    return;
  }
  if (shouldAllowTypeScriptFallback()) {
    return;
  }
  if (await hasBuiltHandlerArtifact(projectDir, normalizedEntry)) {
    return;
  }
  await runTypeScriptBuild(projectDir);
}

async function loadHandler(projectDir: string, entry: string, fresh = false): Promise<UniversalHandler> {
  const handlerPath = await resolveHandlerPath(projectDir, entry);
  const moduleUrl = pathToFileURL(handlerPath).href;
  // Keep native dynamic import semantics in built CJS CLI output.
  // TypeScript downlevels direct `import()` to `require()` for CommonJS, which cannot load `file://` specifiers.
  const dynamicImport = new Function(
    "specifier",
    "return import(specifier);"
  ) as (specifier: string) => Promise<Record<string, unknown>>;
  const moduleSpecifier = fresh
    ? `${moduleUrl}${moduleUrl.includes("?") ? "&" : "?"}v=${Date.now()}-${Math.random().toString(36).slice(2)}`
    : moduleUrl;
  const loadedModule = await dynamicImport(moduleSpecifier);

  const directHandler = loadedModule.handler;
  const defaultExport = loadedModule.default;
  const defaultHandler =
    defaultExport && typeof defaultExport === "object"
      ? (defaultExport as Record<string, unknown>).handler
      : undefined;
  const resolvedHandler =
    typeof directHandler === "function"
      ? directHandler
      : typeof defaultHandler === "function"
        ? defaultHandler
        : typeof defaultExport === "function"
          ? defaultExport
          : undefined;

  if (typeof resolvedHandler !== "function") {
    throw new Error(`expected exported handler function in ${handlerPath}`);
  }
  return resolvedHandler as UniversalHandler;
}

async function loadEvent(projectDir: string, options: ParsedCallOptions): Promise<unknown> {
  if (!options.eventPath) {
    return createEventFromOptions(options);
  }
  const raw = await readFile(resolve(projectDir, options.eventPath), "utf8");
  return JSON.parse(raw);
}

async function invokeWithProvider(
  provider: string,
  handler: UniversalHandler,
  event: unknown
): Promise<LocalInvokeResponse> {
  const normalizeAdapterResponse = (raw: unknown): LocalInvokeResponse => {
    if (!raw || typeof raw !== "object") {
      throw new Error(`${provider}: local adapter returned no response`);
    }

    const response = raw as Record<string, unknown>;
    const statusCode =
      typeof response.statusCode === "number"
        ? response.statusCode
        : typeof response.status === "number"
          ? response.status
          : undefined;

    if (typeof statusCode !== "number") {
      throw new Error(`${provider}: local adapter response is missing status/statusCode`);
    }

    const headers: Record<string, string> = {};
    if (response.headers && typeof response.headers === "object") {
      for (const [key, value] of Object.entries(response.headers as Record<string, unknown>)) {
        headers[key] = String(value);
      }
    }

    const body =
      typeof response.body === "string"
        ? response.body
        : response.body == null
          ? ""
          : JSON.stringify(response.body);

    return {
      statusCode,
      headers,
      body
    };
  };

  switch (provider) {
    case "aws-lambda":
      return normalizeAdapterResponse(await createAwsAdapter(handler)(event));
    case "gcp-functions":
      return normalizeAdapterResponse(await createGcpHttpAdapter(handler)(event));
    case "azure-functions":
      return normalizeAdapterResponse(await createAzureHttpAdapter(handler)({}, event));
    case "vercel":
      return normalizeAdapterResponse(await createVercelHttpAdapter(handler)(event));
    case "netlify":
      return normalizeAdapterResponse(await createNetlifyHttpAdapter(handler)(event));
    case "digitalocean-functions":
      return normalizeAdapterResponse(await createDigitalOceanHttpAdapter(handler)(event));
    case "fly-machines":
      return normalizeAdapterResponse(await createFlyHttpAdapter(handler)(event));
    case "ibm-openwhisk":
      return normalizeAdapterResponse(await createIbmOpenWhiskAdapter(handler)(event));
    case "alibaba-fc":
      return normalizeAdapterResponse(await createAlibabaHttpAdapter(handler)(event));
    case "cloudflare-workers": {
      const adapter = createCloudflareWorkerAdapter(handler);
      const eventRecord = event as Record<string, unknown>;
      const request = new Request(String(eventRecord.url), {
        method: String(eventRecord.method || "GET"),
        headers: (eventRecord.headers as Record<string, string>) || {},
        body: typeof eventRecord.body === "string" ? eventRecord.body : undefined
      });
      const response = await adapter.fetch(request);
      return {
        statusCode: response.status,
        headers: Object.fromEntries(response.headers.entries()),
        body: await response.text()
      };
    }
    default:
      throw new Error(`unsupported provider for local call: ${provider}`);
  }
}

export async function executeLocalCall(projectDir: string, options: LocalCallOptions): Promise<{
  provider: string;
  entry: string;
  request: unknown;
  response: LocalInvokeResponse;
}> {
  const planning = await loadPlanningContext(projectDir, options.config);
  const provider = options.provider || planning.project.providers[0] || "aws-lambda";

  if (!PROVIDER_IDS.includes(provider as (typeof PROVIDER_IDS)[number])) {
    throw new Error(`unknown provider: ${provider}`);
  }

  const parsed: ParsedCallOptions = {
    provider,
    method: (options.method || "GET").toUpperCase(),
    path: options.path || "/hello",
    query: options.query || "",
    body: options.body,
    headers: parseHeaders(options.header || []),
    eventPath: options.event
  };

  const entry = options.entry || planning.project.entry;
  await ensureTypeScriptArtifacts(projectDir, entry);
  const request = await loadEvent(projectDir, parsed);
  const handler = await loadHandler(projectDir, entry, Boolean(options.watch));
  const response = await invokeWithProvider(provider, handler, request);

  return {
    provider,
    entry,
    request,
    response
  };
}

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

async function resolveExecutionContext(
  projectDir: string,
  options: Pick<LocalCallOptions, "config" | "provider" | "entry" | "watch">
): Promise<LocalExecutionContext> {
  const planning = await loadPlanningContext(projectDir, options.config);
  const provider = options.provider || planning.project.providers[0] || "aws-lambda";
  if (!PROVIDER_IDS.includes(provider as (typeof PROVIDER_IDS)[number])) {
    throw new Error(`unknown provider: ${provider}`);
  }

  const entry = options.entry || planning.project.entry;
  await ensureTypeScriptArtifacts(projectDir, entry);
  if (options.watch) {
    return {
      provider,
      entry
    };
  }
  const handler = await loadHandler(projectDir, entry);
  return { provider, entry, handler };
}

function normalizeNodeHeaders(
  headers: Record<string, string | string[] | undefined>
): Record<string, string> {
  const normalized: Record<string, string> = {};
  for (const [key, value] of Object.entries(headers)) {
    if (typeof value === "string") {
      normalized[key] = value;
      continue;
    }
    if (Array.isArray(value)) {
      normalized[key] = value.join(", ");
    }
  }
  return normalized;
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

  const exited = new Promise<void>((resolvePromise) => {
    child.once("exit", () => resolvePromise());
  });

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

async function serveLocalCalls(projectDir: string, options: LocalCallOptions): Promise<void> {
  if (options.event) {
    throw new Error("--event is not supported with --serve");
  }

  const { provider, entry, handler } = await resolveExecutionContext(projectDir, options);
  const host = options.host || "127.0.0.1";
  const port = options.port ?? 8787;
  const extraHeaders = parseHeaders(options.header || []);
  const watchMode = Boolean(options.watch);
  let tscWatch: ChildProcess | undefined;

  const server = createServer(async (request, response) => {
    try {
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
      const invokeResult = await invokeWithProvider(provider, requestHandler, event);

      response.statusCode = invokeResult.statusCode;
      for (const [key, value] of Object.entries(invokeResult.headers)) {
        response.setHeader(key, value);
      }
      response.end(invokeResult.body);
    } catch (serverError) {
      const message = serverError instanceof Error ? serverError.message : String(serverError);
      response.statusCode = 500;
      response.setHeader("content-type", "application/json");
      response.end(JSON.stringify({ error: message }));
    }
  });

  await new Promise<void>((resolvePromise, rejectPromise) => {
    server.once("error", rejectPromise);
    server.listen(port, host, () => {
      resolvePromise();
    });
  });

  const serverAddress = server.address();
  const resolvedPort = serverAddress && typeof serverAddress === "object" ? serverAddress.port : port;
  tscWatch = watchMode && entry.endsWith(".ts") ? startTypeScriptWatch(projectDir) : undefined;
  info(`local call server listening on http://${host}:${resolvedPort}`);
  info(`provider: ${provider}`);
  info(`entry: ${entry}`);
  if (watchMode) {
    info("watch mode enabled");
  }
  if (process.stdin.isTTY) {
    info("press Ctrl+C or type 'exit' and Enter to stop");
  }

  await new Promise<void>((resolvePromise) => {
    let shuttingDown = false;
    let stdinListenerAttached = false;

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

    const onStdinData = (chunk: string | Buffer): void => {
      const text = chunk.toString().trim().toLowerCase();
      if (text === "q" || text === "quit" || text === "exit") {
        shutdown();
      }
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
    .option("--watch", "Reload handler on each request (for TS entry also runs tsc --watch)")
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
