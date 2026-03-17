/**
 * RunFabric Node SDK — handler contract, HTTP adapter, framework adapters.
 */

export type Context = {
  requestId?: string;
  stage?: string;
  functionName?: string;
};

/** Universal request (HTTP-like). When runtime sends this as event, handler returns UniversalResponse. */
export type UniversalRequest = {
  method?: string;
  path?: string;
  query?: Record<string, string | string[]>;
  headers?: Record<string, string>;
  body?: string | undefined;
};

/** Universal response (status, headers, body). Returned by handlers using the universal contract. */
export type UniversalResponse = {
  status: number;
  headers?: Record<string, string>;
  body?: string;
};

/** Handler signature: (event, context?) => response. Event may be UniversalRequest; response may be UniversalResponse or any JSON-serializable object. */
export type UniversalHandler = (
  event: object,
  context?: Context
) => Promise<object> | object;

/** Express app (has use, listen). */
export type ExpressApp = import("express").Application;

/** Fastify instance (has inject). */
export type FastifyInstance = import("fastify").FastifyInstance;

/** Nest application (has getHttpAdapter). */
export type NestApplication = import("@nestjs/common").INestApplication;

export type HandlerOrApp =
  | UniversalHandler
  | ExpressApp
  | FastifyInstance
  | NestApplication;

export type HttpHandler = (
  req: import("http").IncomingMessage,
  res: import("http").ServerResponse
) => void;

export type CreateHandlerResult = HttpHandler & {
  mountExpress: (app: ExpressApp, path?: string, method?: string) => void;
  mountFastify: (fastify: FastifyInstance, options?: { url?: string; method?: string }) => void;
  forNest: () => HttpHandler;
};

/**
 * Create a universal handler from a raw function or an Express/Fastify/Nest app.
 * - createHandler((event, context) => response)
 * - createHandler(expressApp | fastifyInstance | nestApp)
 */
export function createHandler(handlerOrApp: HandlerOrApp): CreateHandlerResult;

export function createHttpHandler(handler: UniversalHandler): HttpHandler;

export function loadHandler(modulePath: string): Promise<UniversalHandler>;

// --- Lifecycle hooks (types + execution) ---

/** Context passed to beforeBuild / afterBuild. */
export type BuildHookContext = {
  cwd?: string;
  config?: unknown;
  [key: string]: unknown;
};

/** Context passed to beforeDeploy / afterDeploy. */
export type DeployHookContext = {
  cwd?: string;
  config?: unknown;
  deployments?: unknown[];
  [key: string]: unknown;
};

/** Describes a deploy failure when available. */
export type DeployFailure = {
  message?: string;
  [key: string]: unknown;
};

/** Lifecycle hook contract (v1). Implement this and export as default; run via Node CLI. */
export type LifecycleHook = {
  name?: string;
  beforeBuild?(context: BuildHookContext): void | Promise<void>;
  afterBuild?(context: BuildHookContext): void | Promise<void>;
  beforeDeploy?(context: DeployHookContext): void | Promise<void>;
  afterDeploy?(context: DeployHookContext): void | Promise<void>;
};

/** Lifecycle phase names. */
export const PHASES: readonly ["beforeBuild", "afterBuild", "beforeDeploy", "afterDeploy"];

/**
 * Load hook modules from paths (e.g. config.hooks). Resolves relative to cwd. Supports ESM (.mjs).
 */
export function loadHookModules(
  hookPaths: string[],
  cwd?: string
): Promise<LifecycleHook[]>;

/**
 * Run a lifecycle phase across loaded hook modules. Call after loadHookModules.
 */
export function runLifecycleHooks(
  hookModules: LifecycleHook[],
  phase: "beforeBuild" | "afterBuild" | "beforeDeploy" | "afterDeploy",
  context: BuildHookContext | DeployHookContext
): Promise<void>;
