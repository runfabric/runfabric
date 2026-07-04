'use strict';

/**
 * RunFabric daemon (runfabricd) HTTP client.
 *
 * Speaks the config API served by platform/daemon/configapi/server.go:
 *   POST /deploy | /remove | /plan | /validate | /resolve | /releases ? config=<rel>&stage=<stage>
 * - No request body: the config path + stage are query params. `config` is
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
};

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

  async #call(op, request = {}) {
    if (!this.available()) {
      return { ok: false, error: 'daemon base URL is not set' };
    }
    const query = new URLSearchParams({ config: request.configPath || 'runfabric.yml' });
    if (request.stage) query.set('stage', request.stage);
    const url = `${this.baseUrl}/${op}?${query.toString()}`;

    const headers = {};
    if (this.apiKey) headers['X-API-Key'] = this.apiKey;
    if (request.providerHeaders) {
      for (const [name, value] of Object.entries(request.providerHeaders)) {
        if (value) headers[name] = value;
      }
    }

    const timeoutMs = request.timeoutMs || this.timeoutsMs[op] || 60_000;
    try {
      const res = await this.fetchImpl(url, {
        method: 'POST',
        headers,
        signal: AbortSignal.timeout(timeoutMs),
      });
      const data = await res.json().catch(() => null);
      if (!res.ok) {
        const error =
          (data && typeof data.error === 'string' && data.error) || res.statusText || `HTTP ${res.status}`;
        return { ok: false, status: res.status, error };
      }
      return { ok: true, status: res.status, data };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : String(e) };
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

module.exports = { DaemonClient, awsProviderHeaders, DEFAULT_TIMEOUTS_MS, CREDENTIALS };
