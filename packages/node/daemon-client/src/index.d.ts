/**
 * RunFabric daemon (runfabricd) HTTP client — typed surface.
 */

/** Per-operation timeouts in milliseconds. */
export interface DaemonTimeoutsMs {
  deploy?: number;
  remove?: number;
  plan?: number;
  validate?: number;
  resolve?: number;
  releases?: number;
}

export interface DaemonClientOptions {
  /** Daemon base URL, e.g. http://127.0.0.1:8766 (trailing slashes stripped). */
  baseUrl: string;
  /** Value for the X-API-Key header (required by non-loopback daemons). */
  apiKey?: string;
  /** Per-operation timeout overrides. */
  timeoutsMs?: DaemonTimeoutsMs;
  /** fetch implementation (defaults to global fetch; Node >= 18). */
  fetch?: typeof fetch;
}

/** One daemon operation request. */
export interface EngineRequest {
  /**
   * runfabric.yml path RELATIVE to the daemon's workspace root, e.g.
   * `<tenant>/<project>/<stage>/runfabric.yml`. Defaults to `runfabric.yml`.
   */
  configPath?: string;
  /** Stage to operate on (daemon default stage when omitted). */
  stage?: string;
  /**
   * Per-request provider credentials as ready-made X-Provider-* headers (see
   * each provider's CredentialVars declaration / plugin.yaml `credentials:`).
   * The daemon applies them to the process env for this operation only.
   */
  providerHeaders?: Record<string, string>;
  /**
   * W3C traceparent to propagate (see newTraceparent). The daemon joins this
   * trace — same trace id end-to-end — and echoes it back as X-Trace-Id.
   */
  traceparent?: string;
  /** Optional W3C tracestate forwarded alongside traceparent. */
  tracestate?: string;
  /** Correlation id sent as X-Request-Id (the daemon generates one if absent). */
  requestId?: string;
  /** Override the operation's default timeout. */
  timeoutMs?: number;
}

/** Uniform result: HTTP and network failures come back as ok:false, never throw. */
export type DaemonResult<T> =
  | { ok: true; status: number; data: T; traceId?: string; requestId?: string }
  | { ok: false; status?: number; error: string; traceId?: string; requestId?: string };

/** Provider DeployResult payload returned verbatim by POST /deploy. */
export interface DeployPayload {
  provider?: string;
  deploymentId?: string;
  outputs?: Record<string, unknown>;
  artifacts?: unknown[];
  metadata?: Record<string, unknown>;
  functions?: Record<string, unknown>;
  [key: string]: unknown;
}

export declare class DaemonClient {
  constructor(options?: DaemonClientOptions);
  readonly baseUrl: string;

  /** True when a base URL is configured. Reachability is verified on first call. */
  available(): boolean;

  /** Deploy the workspace config; data is the provider DeployResult payload. */
  deploy(request?: EngineRequest): Promise<DaemonResult<DeployPayload>>;

  /** Remove the stage's deployed resources. */
  remove(request?: EngineRequest): Promise<DaemonResult<Record<string, unknown> | null>>;

  /** Side-effect-free plan (dry run); data is the plan payload. */
  plan(request?: EngineRequest): Promise<DaemonResult<Record<string, unknown> | null>>;

  /** Side-effect-free config validation; data is {ok:true} on success. */
  validate(request?: EngineRequest): Promise<DaemonResult<{ ok: boolean }>>;

  /** Resolve the config for a stage (interpolation + stage overrides applied). */
  resolve(request?: EngineRequest): Promise<DaemonResult<Record<string, unknown> | null>>;

  /** List releases known to the daemon's state backend. */
  releases(request?: EngineRequest): Promise<DaemonResult<unknown>>;
}

/** One declared credential env var (subset of the Go CredentialVar). */
export interface CredentialVarSpec {
  envKey: string;
  /** X-Provider-* / X-State-* per-request header; absent = env-only. */
  header?: string;
  required?: boolean;
  mirror?: string;
  placeholder?: string;
}

/**
 * The framework's credential declarations, generated from the Go
 * CredentialVars (providers keyed by provider id, state backends by kind).
 */
export declare const CREDENTIALS: {
  providers: Record<string, CredentialVarSpec[]>;
  state: Record<string, CredentialVarSpec[]>;
};

/** Build X-Provider-Aws-* headers from a flat AWS credential object. */
export declare function awsProviderHeaders(creds: {
  accessKeyId: string;
  secretAccessKey: string;
  sessionToken?: string;
  region?: string;
}): Record<string, string>;

/**
 * Generate a W3C traceparent value (new trace id + span id, sampled) for
 * correlating a daemon operation end-to-end. Platforms already running
 * OpenTelemetry should propagate their ambient traceparent instead.
 */
export declare function newTraceparent(): string;

/** Extract the 32-hex trace id from a traceparent value ('' when malformed). */
export declare function traceIdOf(traceparent: string | undefined): string;

export declare const DEFAULT_TIMEOUTS_MS: Required<DaemonTimeoutsMs>;
