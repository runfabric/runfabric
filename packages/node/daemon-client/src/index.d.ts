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
  /**
   * providerOverrides key for multi-cloud deploys (CLI --provider
   * equivalent); "" / absent targets the config's default provider.
   */
  provider?: string;
  /** routerSync only: preview the sync without mutating DNS/LB resources. */
  dryRun?: boolean;
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

  /** Multi-cloud deploy across fabric.targets; data is the fabric state. */
  fabricDeploy(request?: EngineRequest): Promise<DaemonResult<{
    service?: string;
    stage?: string;
    endpoints?: Array<{ provider: string; url: string; updatedAt?: string }>;
    [key: string]: unknown;
  }>>;

  /** Sync the router over the fabric endpoints; data is {routing, result}. */
  routerSync(request?: EngineRequest): Promise<DaemonResult<Record<string, unknown> | null>>;
}

/** One declared credential env var (subset of the Go CredentialVar). */
export interface CredentialVarSpec {
  envKey: string;
  /** X-Provider-* / X-State-* per-request header; absent = env-only. */
  header?: string;
  required?: boolean;
  mirror?: string;
  placeholder?: string;
  /** Env key consulted when envKey is unset (same-cloud provider fallback). */
  fallback?: string;
}

/**
 * The framework's credential declarations, generated from the Go
 * CredentialVars: providers keyed by provider id, state backends by kind,
 * the shared scoped AWS state identity, the router subsystem group,
 * per-router env fallbacks, and token-authenticated secret managers.
 */
export declare const CREDENTIALS: {
  providers: Record<string, CredentialVarSpec[]>;
  state: Record<string, CredentialVarSpec[]>;
  stateAws: CredentialVarSpec[];
  router: CredentialVarSpec[];
  routerPlugins: Record<string, CredentialVarSpec[]>;
  secretManagers: Record<string, CredentialVarSpec[]>;
};

/** Build X-Provider-Aws-* headers from a flat AWS credential object. */
export declare function awsProviderHeaders(creds: {
  accessKeyId: string;
  secretAccessKey: string;
  sessionToken?: string;
  region?: string;
}): Record<string, string>;

/**
 * Build X-State-Aws-* headers: a scoped AWS identity for the s3/dynamodb
 * state backends, independent of the deploy target's identity.
 */
export declare function stateAwsHeaders(creds?: {
  accessKeyId?: string;
  secretAccessKey?: string;
  sessionToken?: string;
  region?: string;
}): Record<string, string>;

/**
 * Build X-Secret-Aws-* headers: a scoped AWS identity for aws-sm:// secret
 * resolution, independent of the deploy target's and state store's identities.
 */
export declare function awsSecretManagerHeaders(creds?: {
  accessKeyId?: string;
  secretAccessKey?: string;
  sessionToken?: string;
  region?: string;
}): Record<string, string>;

/**
 * Build X-Router-* headers so a daemon deploy/remove syncs DNS with the
 * calling project's own router credentials for this one operation.
 */
export declare function routerHeaders(creds?: {
  apiToken?: string;
  zoneId?: string;
  accountId?: string;
}): Record<string, string>;

/**
 * Build X-Secret-Vault-* headers so vault:// secret references resolve with
 * the calling project's Vault identity for this one operation. Cloud secret
 * managers reuse the X-Provider-* group of the same cloud instead.
 */
export declare function vaultSecretManagerHeaders(creds?: {
  token?: string;
  addr?: string;
  namespace?: string;
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
