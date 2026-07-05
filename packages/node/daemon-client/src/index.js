'use strict';

/**
 * RunFabric daemon (runfabricd) HTTP client.
 *
 * Speaks the config API served by platform/daemon/configapi/server.go:
 *   POST /deploy | /remove | /plan | /validate | /resolve | /releases |
 *        /releases/history | /fabric/{deploy,health,targets} |
 *        /router/{sync,history,simulate,verify,shift,restore} |
 *        /state/{list,pull,backup,restore,reconcile,migrate,unlock,lock-steal} |
 *        /invoke | /logs | /metrics/functions | /doctor | /recover
 *        ? config=<rel>&stage=<stage>
 * - Inputs ride query params (config path, stage, op-specific params); only
 *   /invoke carries a JSON body (the invocation payload). `config` is
 *   RELATIVE to the daemon's workspace root; the daemon deploys the
 *   runfabric.yml + built artifacts that live on its own filesystem.
 * - Auth is the `X-API-Key` header (the daemon refuses to bind a non-loopback
 *   address without an API key).
 * - Per-request provider credentials travel as `X-Provider-*` headers (see each
 *   provider's CredentialVars declaration / plugin.yaml `credentials:`); the
 *   daemon applies them to the process env for that one operation only.
 * - Success bodies: /deploy, /plan, /remove, /resolve, /releases return the
 *   engine payload verbatim; /validate returns {ok:true}. Errors return
 *   {ok:false, error} with a 4xx status.
 */

const DEFAULT_TIMEOUTS_MS = {
  deploy: 300_000,
  remove: 120_000,
  plan: 60_000,
  validate: 60_000,
  resolve: 60_000,
  releases: 30_000,
  'releases/history': 30_000,
  // Deploys EVERY fabric.targets provider sequentially.
  'fabric/deploy': 600_000,
  'fabric/health': 60_000,
  'fabric/targets': 30_000,
  'router/sync': 120_000,
  'router/shift': 120_000,
  'router/restore': 120_000,
  invoke: 120_000,
  recover: 300_000,
  'state/migrate': 300_000,
  'state/restore': 120_000,
};

/**
 * Generate a W3C traceparent header value (new trace id + span id, sampled).
 * Pass it per request so the daemon's spans and logs join the caller's trace;
 * platforms already running OpenTelemetry should propagate their ambient
 * traceparent instead of generating one here.
 */
function newTraceparent() {
  const { randomBytes } = require('node:crypto');
  const traceId = randomBytes(16).toString('hex');
  const spanId = randomBytes(8).toString('hex');
  return `00-${traceId}-${spanId}-01`;
}

/** Extract the 32-hex trace id from a traceparent value ('' when malformed). */
function traceIdOf(traceparent) {
  const m = /^[0-9a-f]{2}-([0-9a-f]{32})-[0-9a-f]{16}-[0-9a-f]{2}$/.exec(
    String(traceparent || '').trim().toLowerCase(),
  );
  return m ? m[1] : '';
}

/**
 * Build X-Router-* headers so a daemon deploy/remove syncs DNS with the
 * calling project's OWN router credentials instead of the daemon's ambient
 * env. The daemon clears the whole router group before applying, so a partial
 * set never mixes with ambient values.
 */
function routerHeaders(creds) {
  if (!creds) return {};
  const headers = {};
  if (creds.apiToken) headers['X-Router-Api-Token'] = creds.apiToken;
  if (creds.zoneId) headers['X-Router-Zone-Id'] = creds.zoneId;
  if (creds.accountId) headers['X-Router-Account-Id'] = creds.accountId;
  return headers;
}

/**
 * Build X-Secret-Vault-* headers so secret-manager references (vault://…)
 * resolve with the calling project's Vault identity for this one operation.
 * Cloud secret managers (aws-sm/gcp-sm/azure-kv) need no dedicated headers —
 * they authenticate via the X-Provider-* group of the same cloud.
 */
function vaultSecretManagerHeaders(creds) {
  if (!creds || !creds.token) return {};
  const headers = { 'X-Secret-Vault-Token': creds.token };
  if (creds.addr) headers['X-Secret-Vault-Addr'] = creds.addr;
  if (creds.namespace) headers['X-Secret-Vault-Namespace'] = creds.namespace;
  return headers;
}

/**
 * Build X-State-Aws-* headers: a SCOPED AWS identity for the s3/dynamodb
 * state backends, so state can live in a different AWS account than the
 * deploy target (X-Provider-Aws-*) and the aws-sm secret source
 * (X-Secret-Aws-*). Unset, state falls back to the AWS default chain.
 */
function stateAwsHeaders(creds) {
  if (!creds || !creds.accessKeyId || !creds.secretAccessKey) return {};
  const headers = {
    'X-State-Aws-Access-Key-Id': creds.accessKeyId,
    'X-State-Aws-Secret-Access-Key': creds.secretAccessKey,
  };
  if (creds.sessionToken) headers['X-State-Aws-Session-Token'] = creds.sessionToken;
  if (creds.region) headers['X-State-Aws-Region'] = creds.region;
  return headers;
}

/**
 * Build X-Secret-Aws-* headers: a SCOPED AWS identity for aws-sm:// secret
 * resolution, independent of the deploy target's and the state store's AWS
 * identities.
 */
function awsSecretManagerHeaders(creds) {
  if (!creds || !creds.accessKeyId || !creds.secretAccessKey) return {};
  const headers = {
    'X-Secret-Aws-Access-Key-Id': creds.accessKeyId,
    'X-Secret-Aws-Secret-Access-Key': creds.secretAccessKey,
  };
  if (creds.sessionToken) headers['X-Secret-Aws-Session-Token'] = creds.sessionToken;
  if (creds.region) headers['X-Secret-Aws-Region'] = creds.region;
  return headers;
}

/** Build X-Provider-Aws-* headers from a flat AWS credential object. */
function awsProviderHeaders(creds) {
  if (!creds || !creds.accessKeyId || !creds.secretAccessKey) return {};
  const headers = {
    'X-Provider-Aws-Access-Key-Id': creds.accessKeyId,
    'X-Provider-Aws-Secret-Access-Key': creds.secretAccessKey,
  };
  if (creds.sessionToken) headers['X-Provider-Aws-Session-Token'] = creds.sessionToken;
  if (creds.region) headers['X-Provider-Aws-Region'] = creds.region;
  return headers;
}

class DaemonClient {
  /**
   * @param {object} options
   * @param {string} options.baseUrl  Daemon base URL, e.g. http://127.0.0.1:8766
   * @param {string} [options.apiKey] Value for the X-API-Key header
   * @param {object} [options.timeoutsMs] Per-operation timeout overrides
   * @param {Function} [options.fetch] fetch implementation (defaults to global fetch)
   */
  constructor(options = {}) {
    const url = typeof options.baseUrl === 'string' ? options.baseUrl.trim() : '';
    this.baseUrl = url ? url.replace(/\/+$/, '') : '';
    this.apiKey = options.apiKey || undefined;
    this.timeoutsMs = { ...DEFAULT_TIMEOUTS_MS, ...(options.timeoutsMs || {}) };
    this.fetchImpl = options.fetch || globalThis.fetch;
  }

  /** True when a base URL is configured. Reachability is verified on first call. */
  available() {
    return this.baseUrl.length > 0;
  }

  /** Deploy the workspace config. Returns the provider DeployResult payload as data. */
  deploy(request) {
    return this.#call('deploy', request);
  }

  /** Remove the stage's deployed resources. */
  remove(request) {
    return this.#call('remove', request);
  }

  /** Side-effect-free plan (dry run). */
  plan(request) {
    return this.#call('plan', request);
  }

  /** Side-effect-free config validation. data is {ok:true} on success. */
  validate(request) {
    return this.#call('validate', request);
  }

  /** Resolve the config for a stage (interpolation + stage overrides applied). */
  resolve(request) {
    return this.#call('resolve', request);
  }

  /** List releases known to the daemon's state backend. */
  releases(request) {
    return this.#call('releases', request);
  }

  /** Retained past receipts for one stage, newest first. */
  releaseHistory(request) {
    return this.#call('releases/history', request);
  }

  /**
   * Multi-cloud deploy: deploys to EVERY fabric.targets provider and returns
   * the fabric state ({service, stage, endpoints:[{provider,url},...]}).
   * Forward one X-Provider-* group per target cloud in providerHeaders.
   */
  fabricDeploy(request) {
    return this.#call('fabric/deploy', request);
  }

  /**
   * Router over the fabric endpoints: syncs the multi-cloud routing config
   * (failover/latency/round-robin) through the configured router plugin.
   * Pass {dryRun:true} to preview. Router creds ride X-Router-* headers or
   * fall back to the same-cloud provider group per the declared fallbacks.
   */
  routerSync(request) {
    return this.#call('router/sync', request);
  }

  /** Fabric endpoint health: per-provider endpoints with health status. */
  fabricHealth(request) {
    return this.#call('fabric/health', request);
  }

  /** Fabric target list: the config's fabric.targets provider keys. */
  fabricTargets(request) {
    return this.#call('fabric/targets', request);
  }

  /**
   * Invoke one deployed function (or `workflow:<name>` orchestration target).
   * {function} is required; {payload} (JSON-serializable) rides the body.
   */
  invoke(request = {}) {
    return this.#call('invoke', {
      ...request,
      params: { function: request.function, ...(request.params || {}) },
      body: request.payload,
    });
  }

  /** Provider + local logs for one function ({function}) or all ("" = all). */
  logs(request = {}) {
    return this.#call('logs', {
      ...request,
      params: { function: request.function, service: request.service, ...(request.params || {}) },
    });
  }

  /**
   * Per-function metrics from the provider (NOT the daemon's own Prometheus
   * /metrics). {all:true} includes every function.
   */
  functionMetrics(request = {}) {
    return this.#call('metrics/functions', {
      ...request,
      params: { service: request.service, all: request.all ? '1' : undefined, ...(request.params || {}) },
    });
  }

  /** Traces aggregated by service/stage from the provider. */
  traces(request = {}) {
    return this.#call('traces', {
      ...request,
      params: { service: request.service, all: request.all ? '1' : undefined, ...(request.params || {}) },
    });
  }

  /** Backend + provider readiness checks ({backend, provider} payload). */
  doctor(request) {
    return this.#call('doctor', request);
  }

  /**
   * Recover an unfinished transaction journal.
   * {mode}: rollback|resume|inspect (default rollback); {dryRun:true} previews.
   */
  recover(request = {}) {
    return this.#call('recover', {
      ...request,
      params: { mode: request.mode, ...(request.params || {}) },
    });
  }

  /**
   * State-backend operation: list|pull|backup|restore|reconcile|migrate|
   * unlock|lock-steal. Op-specific inputs ride {params} (out, file, from, to,
   * force) — paths are relative to the daemon workspace.
   */
  stateOp(op, request = {}) {
    return this.#call(`state/${op}`, request);
  }

  /**
   * Router operation over the recorded fabric state: history|simulate|verify|
   * shift|restore. Op-specific inputs ride {params} (requests, down, window,
   * percent, snapshot, latest) plus the shared {provider}/{dryRun} fields.
   */
  routerOp(op, request = {}) {
    return this.#call(`router/${op}`, request);
  }

  /**
   * Start a durable workflow run. {name} is the workflow name from the
   * deployed config; {payload} (JSON-serializable) is the run input; {runId}
   * optionally pins a deterministic run id.
   */
  workflowRun(request = {}) {
    return this.#call('workflow/run', {
      ...request,
      params: { name: request.name, runId: request.runId, ...(request.params || {}) },
      body: request.payload ?? {},
    });
  }

  /** Load one workflow run (status, steps, outputs). */
  workflowStatus(request = {}) {
    return this.#call('workflow/status', {
      ...request,
      params: { runId: request.runId, ...(request.params || {}) },
    });
  }

  /** Cancel a running workflow run. */
  workflowCancel(request = {}) {
    return this.#call('workflow/cancel', {
      ...request,
      params: { runId: request.runId, ...(request.params || {}) },
    });
  }

  /** Replay a workflow run from one step (journal-backed). */
  workflowReplay(request = {}) {
    return this.#call('workflow/replay', {
      ...request,
      params: { runId: request.runId, step: request.step, ...(request.params || {}) },
    });
  }

  /**
   * Resolve a paused human-approval step and resume the run. {decision} is
   * approve|reject; {step} is optional (defaults to the paused step); {reviewer}
   * is recorded on the step output for the audit trail.
   */
  workflowApprove(request = {}) {
    return this.#call('workflow/approve', {
      ...request,
      params: {
        runId: request.runId,
        step: request.step,
        decision: request.decision,
        reviewer: request.reviewer,
        ...(request.params || {}),
      },
    });
  }

  /** List the stage's most recent workflow runs. */
  workflowRuns(request = {}) {
    return this.#call('workflow/runs', {
      ...request,
      params: { limit: request.limit, ...(request.params || {}) },
    });
  }

  async #call(op, request = {}) {
    if (!this.available()) {
      return { ok: false, error: 'daemon base URL is not set' };
    }
    const query = new URLSearchParams({ config: request.configPath || 'runfabric.yml' });
    if (request.stage) query.set('stage', request.stage);
    // providerOverrides key for multi-cloud (CLI --provider equivalent);
    // doubles as the canary target for router/shift.
    if (request.provider) query.set('provider', request.provider);
    if (request.dryRun) query.set('dryRun', '1');
    // Op-specific query params (function, service, mode, out, file, ...).
    for (const [name, value] of Object.entries(request.params || {})) {
      if (value !== undefined && value !== null && value !== '') query.set(name, String(value));
    }
    const url = `${this.baseUrl}/${op}?${query.toString()}`;

    const headers = {};
    if (this.apiKey) headers['X-API-Key'] = this.apiKey;
    // Distributed-trace + request correlation: the daemon joins the trace in
    // traceparent (echoing X-Trace-Id) and echoes/generates X-Request-Id.
    if (request.traceparent) headers['traceparent'] = request.traceparent;
    if (request.tracestate) headers['tracestate'] = request.tracestate;
    if (request.requestId) headers['X-Request-Id'] = request.requestId;
    if (request.providerHeaders) {
      for (const [name, value] of Object.entries(request.providerHeaders)) {
        if (value) headers[name] = value;
      }
    }

    let body;
    if (request.body !== undefined && request.body !== null) {
      body = JSON.stringify(request.body);
      headers['Content-Type'] = 'application/json';
    }

    const timeoutMs = request.timeoutMs || this.timeoutsMs[op] || 60_000;
    try {
      const res = await this.fetchImpl(url, {
        method: 'POST',
        headers,
        body,
        signal: AbortSignal.timeout(timeoutMs),
      });
      const traceId = res.headers?.get?.('x-trace-id') || traceIdOf(request.traceparent) || undefined;
      const requestId = res.headers?.get?.('x-request-id') || request.requestId || undefined;
      const data = await res.json().catch(() => null);
      if (!res.ok) {
        const error =
          (data && typeof data.error === 'string' && data.error) || res.statusText || `HTTP ${res.status}`;
        return { ok: false, status: res.status, error, traceId, requestId };
      }
      return { ok: true, status: res.status, data, traceId, requestId };
    } catch (e) {
      return {
        ok: false,
        error: e instanceof Error ? e.message : String(e),
        traceId: traceIdOf(request.traceparent) || undefined,
        requestId: request.requestId,
      };
    }
  }
}

/**
 * The framework's credential declarations (providers + state backends),
 * generated from the Go CredentialVars and kept in sync by a framework test.
 * Downstream platforms should validate their credential catalogs against this
 * instead of hardcoding header/env names.
 */
const CREDENTIALS = require('./credentials.json');

module.exports = {
  DaemonClient,
  awsProviderHeaders,
  stateAwsHeaders,
  awsSecretManagerHeaders,
  routerHeaders,
  vaultSecretManagerHeaders,
  newTraceparent,
  traceIdOf,
  DEFAULT_TIMEOUTS_MS,
  CREDENTIALS,
};
