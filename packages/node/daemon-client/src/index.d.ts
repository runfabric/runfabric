/**
 * RunFabric daemon (runfabricd) HTTP client — typed surface.
 */

/** Per-operation timeouts in milliseconds (keyed by endpoint path, e.g. 'fabric/deploy'). */
export interface DaemonTimeoutsMs {
  deploy?: number;
  remove?: number;
  plan?: number;
  validate?: number;
  resolve?: number;
  releases?: number;
  invoke?: number;
  recover?: number;
  [endpoint: string]: number | undefined;
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
  /** Mutating router/state/recover ops: preview without applying changes. */
  dryRun?: boolean;
  /** Override the operation's default timeout. */
  timeoutMs?: number;
  /**
   * Op-specific query params (e.g. stateOp's out/file/from/to/force,
   * routerOp's requests/down/window/percent/snapshot/latest).
   */
  params?: Record<string, string | number | boolean | undefined>;
}

/** invoke(): EngineRequest plus the function target and JSON payload. */
export interface InvokeRequest extends EngineRequest {
  /** Function name or `workflow:<name>` orchestration target. Required. */
  function: string;
  /** JSON-serializable invocation payload (sent as the request body). */
  payload?: unknown;
}

/** logs(): EngineRequest plus function/service filters. */
export interface LogsRequest extends EngineRequest {
  /** Function to fetch logs for ("" / absent = all functions). */
  function?: string;
  /** Service scope guard (must match the config's service when set). */
  service?: string;
}

/** functionMetrics(): EngineRequest plus service/all filters. */
export interface FunctionMetricsRequest extends EngineRequest {
  service?: string;
  /** Include every function (not just the deployed stage's default view). */
  all?: boolean;
}

/** recover(): EngineRequest plus the recovery mode. */
export interface RecoverRequest extends EngineRequest {
  /** rollback | resume | inspect (default rollback). */
  mode?: string;
}

/** State-backend operations served under POST /state/{op}. */
export type StateOpName =
  | 'list'
  | 'pull'
  | 'backup'
  | 'restore'
  | 'reconcile'
  | 'migrate'
  | 'unlock'
  | 'lock-steal';

/** Router operations over recorded fabric state under POST /router/{op}. */
export type RouterOpName = 'history' | 'simulate' | 'verify' | 'shift' | 'restore';

/** workflowRun(): EngineRequest plus the workflow name, input, and run id. */
export interface WorkflowRunRequest extends EngineRequest {
  /** Workflow name from the deployed config. Required. */
  name: string;
  /** JSON-serializable run input (sent as the request body). */
  payload?: unknown;
  /** Optional deterministic run id. */
  runId?: string;
}

/** workflowStatus()/workflowCancel()/workflowReplay(): address a run. */
export interface WorkflowRunRefRequest extends EngineRequest {
  runId: string;
  /** workflowReplay only: step id to replay from. */
  step?: string;
}

/** workflowRuns(): EngineRequest plus a result cap. */
export interface WorkflowRunsRequest extends EngineRequest {
  limit?: number;
}

/** workflowApprove(): resolve a paused human-approval step and resume. */
export interface WorkflowApproveRequest extends EngineRequest {
  runId: string;
  /** approve | reject (approved/rejected accepted too). */
  decision: string;
  /** Step id awaiting approval; defaults to the run's paused step. */
  step?: string;
  /** Recorded on the step output for the audit trail. */
  reviewer?: string;
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

  /** Retained past receipts for one stage, newest first. */
  releaseHistory(request?: EngineRequest): Promise<DaemonResult<unknown>>;

  /** Multi-cloud deploy across fabric.targets; data is the fabric state. */
  fabricDeploy(request?: EngineRequest): Promise<DaemonResult<{
    service?: string;
    stage?: string;
    endpoints?: Array<{ provider: string; url: string; updatedAt?: string }>;
    [key: string]: unknown;
  }>>;

  /** Sync the router over the fabric endpoints; data is {routing, result}. */
  routerSync(request?: EngineRequest): Promise<DaemonResult<Record<string, unknown> | null>>;

  /** Probe every recorded fabric endpoint; data is the fabric state with health. */
  fabricHealth(request?: EngineRequest): Promise<DaemonResult<Record<string, unknown> | null>>;

  /** List the config's fabric.targets provider keys; data is {targets}. */
  fabricTargets(request?: EngineRequest): Promise<DaemonResult<{ targets?: string[] }>>;

  /** Invoke one deployed function (or workflow target) with a JSON payload. */
  invoke(request: InvokeRequest): Promise<DaemonResult<unknown>>;

  /** Provider + local logs for one function ("" = all functions). */
  logs(request?: LogsRequest): Promise<DaemonResult<unknown>>;

  /** Per-function metrics from the provider (not the daemon's Prometheus /metrics). */
  functionMetrics(request?: FunctionMetricsRequest): Promise<DaemonResult<unknown>>;

  /** Traces aggregated by service/stage from the provider. */
  traces(request?: FunctionMetricsRequest): Promise<DaemonResult<unknown>>;

  /** Backend + provider readiness checks; data is {backend, provider}. */
  doctor(request?: EngineRequest): Promise<DaemonResult<Record<string, unknown> | null>>;

  /** Recover an unfinished transaction journal (dryRun previews). */
  recover(request?: RecoverRequest): Promise<DaemonResult<unknown>>;

  /** State-backend op; op-specific inputs ride request.params. */
  stateOp(op: StateOpName, request?: EngineRequest): Promise<DaemonResult<unknown>>;

  /** Router op over recorded fabric state; op-specific inputs ride request.params. */
  routerOp(op: RouterOpName, request?: EngineRequest): Promise<DaemonResult<unknown>>;

  /** Start a durable workflow run; data is {workflow, source, run}. */
  workflowRun(request: WorkflowRunRequest): Promise<DaemonResult<unknown>>;

  /** Load one workflow run (status, steps, outputs). */
  workflowStatus(request: WorkflowRunRefRequest): Promise<DaemonResult<unknown>>;

  /** Cancel a running workflow run. */
  workflowCancel(request: WorkflowRunRefRequest): Promise<DaemonResult<unknown>>;

  /** Replay a workflow run from one step (journal-backed). */
  workflowReplay(request: WorkflowRunRefRequest): Promise<DaemonResult<unknown>>;

  /** Resolve a paused human-approval step (approve|reject) and resume the run. */
  workflowApprove(request: WorkflowApproveRequest): Promise<DaemonResult<unknown>>;

  /** List the stage's most recent workflow runs. */
  workflowRuns(request?: WorkflowRunsRequest): Promise<DaemonResult<unknown>>;
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
