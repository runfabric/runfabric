import { once } from "node:events";
import { createServer, type Server } from "node:http";
import type { UniversalHandler, UniversalRequest, UniversalResponse } from "@runfabric/core";

interface FastifyInjectResponseLike {
  statusCode: number;
  headers?: Record<string, unknown>;
  body?: string;
  payload?: string;
}

interface FastifyInjectOptionsLike {
  method: string;
  url: string;
  headers?: Record<string, string>;
  payload?: unknown;
}

export interface FastifyLikeInstance {
  inject: (options: FastifyInjectOptionsLike) => Promise<FastifyInjectResponseLike>;
}

export interface NestLikeHttpAdapter {
  getType?: () => string;
  getInstance: () => unknown;
}

export interface NestLikeApplication {
  getHttpAdapter: () => NestLikeHttpAdapter;
}

type ExpressLikeApplication = (
  req: unknown,
  res: unknown,
  next?: (error?: unknown) => void
) => unknown;

interface ExpressServerState {
  server: Server;
  origin: string;
}

function normalizeHeaders(headers: Record<string, unknown> | undefined): Record<string, string> {
  const normalized: Record<string, string> = {};
  if (!headers) {
    return normalized;
  }

  for (const [key, value] of Object.entries(headers)) {
    if (typeof value === "string") {
      normalized[key] = value;
      continue;
    }
    if (Array.isArray(value)) {
      normalized[key] = value.map((item) => String(item)).join(", ");
      continue;
    }
    if (value !== undefined && value !== null) {
      normalized[key] = String(value);
    }
  }

  return normalized;
}

function buildUrlPath(path: string, query: Record<string, string | string[]>): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query || {})) {
    if (Array.isArray(value)) {
      for (const item of value) {
        params.append(key, item);
      }
      continue;
    }
    params.set(key, value);
  }
  const encodedQuery = params.toString();
  return encodedQuery ? `${path}?${encodedQuery}` : path;
}

function toPayload(body: string | undefined, headers: Record<string, string>): unknown {
  if (!body) {
    return undefined;
  }

  const contentType = headers["content-type"] || headers["Content-Type"];
  if (typeof contentType === "string" && contentType.toLowerCase().includes("application/json")) {
    try {
      return JSON.parse(body);
    } catch {
      return body;
    }
  }

  return body;
}

function isFastifyInstance(value: unknown): value is FastifyLikeInstance {
  return typeof value === "object" && value !== null && typeof (value as FastifyLikeInstance).inject === "function";
}

function isNestApplication(value: unknown): value is NestLikeApplication {
  return (
    typeof value === "object" &&
    value !== null &&
    typeof (value as NestLikeApplication).getHttpAdapter === "function"
  );
}

function isExpressApplication(value: unknown): value is ExpressLikeApplication {
  return typeof value === "function";
}

async function ensureExpressServer(
  app: ExpressLikeApplication,
  state: { current?: ExpressServerState }
): Promise<ExpressServerState> {
  if (state.current) {
    return state.current;
  }

  const server = createServer((request, response) => {
    const onError = (errorValue: unknown): void => {
      const message = errorValue instanceof Error ? errorValue.message : String(errorValue);
      response.statusCode = 500;
      response.setHeader("content-type", "application/json");
      response.end(JSON.stringify({ error: message }));
    };

    try {
      const maybePromise = app(request, response, (errorValue) => {
        if (errorValue) {
          onError(errorValue);
        }
      });

      if (maybePromise && typeof (maybePromise as Promise<unknown>).then === "function") {
        (maybePromise as Promise<unknown>).catch(onError);
      }
    } catch (errorValue) {
      onError(errorValue);
    }
  });

  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  server.unref();

  const address = server.address();
  if (!address || typeof address !== "object") {
    throw new Error("failed to bind express wrapper server");
  }

  const serverState: ExpressServerState = {
    server,
    origin: `http://127.0.0.1:${address.port}`
  };
  state.current = serverState;
  return serverState;
}

function createExpressHandler(app: unknown): UniversalHandler {
  if (!isExpressApplication(app)) {
    throw new Error("createHandler expects an express-compatible app function");
  }

  const state: { current?: ExpressServerState } = {};

  return async (request: UniversalRequest): Promise<UniversalResponse> => {
    const server = await ensureExpressServer(app, state);
    const urlPath = buildUrlPath(request.path || "/", request.query || {});

    const response = await fetch(`${server.origin}${urlPath}`, {
      method: request.method || "GET",
      headers: request.headers || {},
      body: request.body
    });

    return {
      status: response.status,
      headers: Object.fromEntries(response.headers.entries()),
      body: await response.text()
    };
  };
}

function createFastifyHandler(app: unknown): UniversalHandler {
  if (!isFastifyInstance(app)) {
    throw new Error("createHandler expects a fastify-compatible instance with inject()");
  }

  return async (request: UniversalRequest): Promise<UniversalResponse> => {
    const headers = normalizeHeaders(request.headers as Record<string, unknown> | undefined);
    const injected = await app.inject({
      method: request.method || "GET",
      url: buildUrlPath(request.path || "/", request.query || {}),
      headers,
      payload: toPayload(request.body, headers)
    });

    const body = typeof injected.body === "string"
      ? injected.body
      : typeof injected.payload === "string"
        ? injected.payload
        : "";

    return {
      status: injected.statusCode || 200,
      headers: normalizeHeaders(injected.headers),
      body
    };
  };
}

function createNestHandler(app: NestLikeApplication): UniversalHandler {
  if (!app || typeof app.getHttpAdapter !== "function") {
    throw new Error("createHandler expects a nest application with getHttpAdapter()");
  }

  const adapter = app.getHttpAdapter();
  const adapterType = typeof adapter.getType === "function" ? adapter.getType() : undefined;
  const instance = adapter.getInstance();

  if (adapterType === "fastify" || isFastifyInstance(instance)) {
    return createFastifyHandler(instance);
  }
  if (adapterType === "express" || isExpressApplication(instance)) {
    return createExpressHandler(instance);
  }

  throw new Error(`unsupported nest http adapter type: ${adapterType || "unknown"}`);
}

export function createHandler(app: unknown): UniversalHandler {
  if (isNestApplication(app)) {
    return createNestHandler(app);
  }
  if (isFastifyInstance(app)) {
    return createFastifyHandler(app);
  }
  if (isExpressApplication(app)) {
    return createExpressHandler(app);
  }

  throw new Error(
    "createHandler expects one of: nest app (getHttpAdapter), fastify instance (inject), or express app function"
  );
}
