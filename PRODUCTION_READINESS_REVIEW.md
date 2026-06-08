# RunFabric — Production Readiness Review

**Reviewers' perspective:** Principal Golang Engineer · Staff Software Architect · Security Reviewer · SRE
**Date:** 2026-06-07
**Branch:** `main` (`8190e15`)

**Scope reviewed:** ~84k LOC Go across `cmd/`, `internal/cli/`, `platform/` (core, workflow, daemon, state, deploy, extensions). Multi-module workspace (`go.work`). Three binaries:

- `runfabric` — CLI control plane
- `runfabricd` — daemon / HTTP API
- `runfabricw` — "worker"

**Target the service must support:** 100k daily users · multiple worker instances · multiple API instances · Kubernetes deployment · zero-downtime releases.

---

## Executive Summary

This is an architecturally mature, security-conscious codebase (atomic state writes, owner-only permissions, signature/checksum verification, path-traversal guards, loopback-by-default binding). But it is **not ready** for the stated target.

The deployment model is fundamentally **single-node / local-filesystem**:

- Workflow run state is local JSON with **no distributed locking or CAS**.
- HTTP servers have **no timeouts and no graceful shutdown**.
- The rate limiter is **broken and per-instance**.
- The shipped Docker image **cannot start with its default arguments**.
- The "worker" is **not a queue consumer** at all.

> ⚠️ **Reality check on the framing:** This is a CLI + deploy-orchestrator, *not* a REST-API-with-background-workers product. There is **no message queue, no DLQ, no job consumer, no horizontal worker pool**. `runfabricw` is just the CLI with most commands disabled (`internal/cli/worker/root.go`). Several "Worker Review" / "API Review" criteria therefore have *nothing to evaluate* — which is itself the finding: **if you need multi-instance workers, the substrate to build them does not exist yet.**

---

## Production Launch Blockers (fix before any multi-instance deploy)

### 🔴 CRITICAL — Default Docker image cannot start

- **File / Package / Function:** `Dockerfile.daemon` (ENTRYPOINT) + `platform/daemon/server/server.go:133` `RequireAuthForBind`
- **Problem:** The image's `ENTRYPOINT` is `runfabricd --address 0.0.0.0 --port 8766` with no `--api-key`. `RequireAuthForBind("0.0.0.0", "")` returns an error because `0.0.0.0` is not loopback and no key is set. The daemon exits non-zero immediately.
- **Impact:** `docker run runfabric-daemon` and `docker compose up` (which also publishes `8766:8766`) both crash-loop out of the box. In K8s this is `CrashLoopBackOff` on first deploy.
- **Recommendation:** Either (a) make the image require an injected `RUNFABRIC_API_KEY` and pass it through, or (b) default the entrypoint to loopback and document the explicit opt-in. Add a CI smoke test that actually runs the built image.

```dockerfile
# Read key from env so the default image can bind 0.0.0.0 safely.
ENTRYPOINT ["/usr/local/bin/runfabricd", "--address", "0.0.0.0", "--port", "8766"]
# and in daemoncmd: if apiKey == "" { apiKey = os.Getenv("RUNFABRIC_API_KEY") }
```

### 🔴 CRITICAL — No graceful shutdown anywhere in the daemon/API

- **File / Package / Function:** `internal/cli/daemoncmd/daemon.go:197`, `internal/cli/configuration/config_api.go:43`, `platform/daemon/server/socket.go:47`
- **Problem:** All three serve via bare `http.ListenAndServe` / `srv.Serve` with no `signal.Notify` + `srv.Shutdown(ctx)`. On SIGTERM (every K8s rolling update, every pod eviction) the process is killed mid-request.
- **Impact:** In-flight `POST /deploy` / `/remove` are severed mid-write. Because deploy mutates state and infra, this can leave **half-applied deployments and stale locks** that block the next run. Zero-downtime releases are impossible.
- **Recommendation:** Construct an explicit `*http.Server`, trap SIGTERM/SIGINT, and call `Shutdown` with a drain timeout. Hold the lease/lock until in-flight ops finish.

```go
srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
go func() {
    <-sigCh
    ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
    defer cancel()
    _ = srv.Shutdown(ctx)
}()
if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
    return err
}
```

### 🔴 CRITICAL — HTTP servers have no timeouts (Slowloris / resource exhaustion)

- **File / Package / Function:** All server constructions — `config_api.go:31`, `socket.go:46`, `workflow/app/call_local.go:49`, and the `ListenAndServe` in `daemon.go` / `dashboard.go`.
- **Problem:** No `ReadTimeout`, `ReadHeaderTimeout`, `WriteTimeout`, or `IdleTimeout`. No `http.MaxBytesReader` on any handler (grep confirms zero usages).
- **Impact:** A handful of slow/idle connections exhaust the listener; large request bodies are read unbounded. Trivial DoS once exposed.
- **Recommendation:** Set timeouts on every `http.Server`; wrap bodies with `http.MaxBytesReader`.

### 🔴 CRITICAL — Workflow run state has no concurrency control (lost updates / double execution)

- **File / Package / Function:** `platform/workflow/runtime/workflow_runtime.go:312-454` (`executeStep`) + `platform/core/state/core/runs.go:122` (`SaveWorkflowRun`)
- **Problem:** Execution is a repeated `LoadWorkflowRun → mutate → SaveWorkflowRun` read-modify-write with **no lock and no version/CAS**. `WriteStateFile` is atomic *per write* but provides no mutual exclusion between readers/writers. The deploy path *does* use `ManagedLock`/heartbeat — but the **workflow runtime does not**.
- **Impact:** Two `ResumeRun` calls for the same run (retry, two pods, a re-trigger) interleave and clobber each other's status — steps run twice, attempt counters corrupt, a finished run flips back to running. Core blocker for "multiple worker instances."
- **Recommendation:** Acquire the run lock around `ResumeRun`, or add an optimistic-concurrency token (a monotonically increasing `Version`/ETag persisted in the run; reject save if it changed). Reuse the existing `locking` package rather than inventing a third lock.

### 🔴 CRITICAL — State is local-filesystem only; not shareable across instances

- **File / Package / Function:** `platform/core/state/core/runs.go`, `WriteStateFile` → `.runfabric/...` on local disk
- **Problem:** Workflow run state and locks live under `.runfabric/` on the local FS. There are S3/DynamoDB *deploy-state* backends, but workflow runs and `FileBackend`/`FileLock` locks are local. File locks give **zero** mutual exclusion across pods (and are unsafe on NFS).
- **Impact:** Each replica has its own state; runs are invisible/uncoordinated across instances; "lock already held" protection is per-node. Horizontal scaling of execution is impossible as built.
- **Recommendation:** Route workflow run state and locking through the same pluggable backend abstraction the deploy state uses (DynamoDB conditional writes / S3 + a real distributed lock). Treat local FS as the dev-only backend.

---

## Security Review

### 🔴 HIGH — Resolved secrets cached in Redis in cleartext

- **File / Package / Function:** `platform/daemon/server/cache.go:136-186` (`/resolve` in the `cacheable` set) + `platform/daemon/configapi/server.go:122`
- **Problem:** `/resolve` returns the *resolved* config (secret values substituted — the path-traversal guard comment itself says "return resolved secrets"). The cache middleware stores that full body in Redis for 5 minutes, unencrypted, keyed only by `sha256(body)+stage` — not by identity/API key.
- **Impact:** Plaintext secrets at rest in a shared cache; anyone with Redis access (or a shared cache-prefix collision across workspaces) reads them. Secret lifetime now bounded by Redis eviction, not by the request.
- **Recommendation:** Never cache `/resolve`. If response caching is needed, cache only non-secret-bearing endpoints, and key cache entries by authenticated identity.

### 🔴 HIGH — API-key comparison is not constant-time

- **File / Package / Function:** `platform/daemon/configapi/server.go:56`
- **Problem:** `r.Header.Get("X-API-Key") != s.APIKey` — byte comparison short-circuits, leaking length/prefix via timing.
- **Impact:** Timing side-channel on the only auth credential.
- **Recommendation:** `subtle.ConstantTimeCompare([]byte(got), []byte(s.APIKey)) != 1`.

### 🟠 MEDIUM — Single static shared API key; no authn/z, no rotation, no tenancy

- **File / Package / Function:** `platform/daemon/configapi/server.go`
- **Problem:** One process-wide API key gates every mutating operation (deploy/remove). No per-user identity, no RBAC, no rotation, no audit of *who* called.
- **Impact:** For 100k users this is unworkable — no least privilege, no revocation, blast radius = everything.
- **Recommendation:** Move to per-principal tokens (JWT/OIDC) with scopes; record principal in the audit log; support rotation.

### 🟠 MEDIUM — SSRF guard overstates its protection

- **File / Package / Function:** `platform/extensions/application/external/registry.go:481` `validateSecureURL`
- **Problem:** The comment claims it "constrains server-side fetches to explicit, non-internal hosts," but it only enforces `https` (or http-on-loopback). It happily fetches `https://169.254.169.254/...`, `https://10.x`, internal service names, etc.
- **Impact:** If a registry response (or `RUNFABRIC_REGISTRY_URL`) is attacker-influenced, fetches can hit cloud metadata / internal endpoints over HTTPS. Mitigated today by checksum+signature, but the SSRF surface is real and the comment is misleading.
- **Recommendation:** Add a registry-host allowlist and block link-local/private/metadata ranges, or resolve+validate the IP before dialing.

### 🟠 MEDIUM — Supply chain: only a `local-dev` signing key is trusted; unsigned artifacts install with checksum-only (TOFU)

- **File / Package / Function:** `platform/extensions/application/external/registry.go:307-339`, `trustedPublicKey` (only `local-dev`)
- **Problem:** No production public key exists. Artifacts without a signature install as long as the publisher isn't flagged `verified`; the checksum comes from the *same* registry response, so a compromised/malicious registry can serve matching bytes+checksum.
- **Impact:** Plugin binaries run with `Network+Cloud+Env` permissions; weak provenance = supply-chain RCE risk on operator machines/CI.
- **Recommendation:** Ship a real trust root; require signatures for all non-local installs; pin keys.

### 🟢 LOW — OTLP exporter always `WithInsecure()`, even for https endpoints

- **File / Package / Function:** `platform/observability/telemetry` `Init`
- **Problem:** It strips the scheme and forces insecure transport; traces (which may carry config/route attributes) go in plaintext.
- **Recommendation:** Use TLS when the endpoint is https; only `WithInsecure` for explicit localhost.

**Positives:** atomic owner-only state writes (`atomic_write.go`) are textbook-correct (temp + fsync + rename + dir fsync); path-traversal confinement in `configPath`; loopback-default + `RequireAuthForBind`; `.env` is gitignored and untracked; lock owner-token uses `crypto/rand`.

---

## Concurrency & Reliability

### 🟠 MEDIUM — Rate limiter is effectively non-functional and leaks memory

- **File / Package / Function:** `platform/daemon/configapi/server.go:55-81` `authorizeAndLimit`
- **Problems (three in one):**
  1. Keyed by `r.RemoteAddr` = `IP:port`. The ephemeral port differs per TCP connection, so each connection gets its own bucket — an attacker opening new connections bypasses the limit entirely; keep-alive clients get one bucket per connection.
  2. The `requests` map is **never pruned** — entries accumulate per `IP:port` forever → unbounded memory growth (a slow DoS itself).
  3. In-memory + per-instance → with multiple API replicas the limit is multiplied by replica count and resets on restart.
- **Impact:** Rate limiting provides little real protection and adds a memory-leak DoS vector.
- **Recommendation:** Key by client IP (`net.SplitHostPort` → host; honor `X-Forwarded-For` only behind a trusted proxy); evict idle buckets (or use a sliding-window/token-bucket lib); for multi-instance use the Redis you already depend on.

### 🟠 MEDIUM — Unbounded request body read when cache is enabled (OOM)

- **File / Package / Function:** `platform/daemon/server/cache.go:151` `io.ReadAll(r.Body)`
- **Problem:** The config handlers don't read the body, but the cache middleware reads the *entire* body into memory to hash it, with no size cap.
- **Impact:** Enabling Redis cache turns any large POST into an OOM vector.
- **Recommendation:** `io.ReadAll(io.LimitReader(r.Body, maxBody))` and reject oversize.

### 🟠 MEDIUM — Retry backoff ignores context cancellation; backoff is constant, not exponential, no jitter

- **File / Package / Function:** `platform/workflow/runtime/workflow_mcp_runtime.go:146-154` (`time.Sleep`) and `platform/workflow/runtime/workflow_runtime.go:413-416` (`r.Sleep(backoff)` with fixed `BackoffMs`)
- **Problem:** Both retry loops `Sleep` without selecting on `ctx.Done()`, so a cancelled/timed-out run keeps sleeping and retrying. Backoff is a fixed duration — no exponential growth, no jitter.
- **Impact:** Slow, uncancellable shutdown; on cloud 429s/5xx, fixed backoff across many runs causes thundering-herd retries.
- **Recommendation:** `select { case <-ctx.Done(): return ctx.Err(); case <-time.After(b): }`; make backoff exponential with full jitter and a cap.

### 🟢 LOW — `callWithRetry` can loop forever if `ShouldRetry` never declines

- **File / Package / Function:** `platform/workflow/runtime/workflow_mcp_runtime.go:147` — `for attempt := 1; ;` has no hard cap independent of the strategy.
- **Recommendation:** Add an absolute max-attempts ceiling as a safety net.

**Positives:** `StartHeartbeat` (`platform/deploy/controlplane/cluster/heartbeat.go`) is a clean, leak-free goroutine (buffered errCh, `ctx.Done()`, ticker stopped). The `executeStep` crash-reconciliation logic (re-checking step status after a persisted "running") is thoughtful.

---

## API Review

- 🟠 **No request body size limits, no API versioning** on the daemon API. Routes are `/validate`, `/deploy`, … with no `/v1` prefix — breaking changes have nowhere to go. (The *registry client* uses `/v1/...`, but the daemon's own API does not.)
- 🟠 **`/deploy` and `/remove` are not idempotent and take no idempotency key.** A client retry after a dropped connection (very likely given no graceful shutdown) re-runs a mutation.
- 🟢 **Health checks conflate liveness and readiness.** `platform/daemon/server/server.go:102` `/healthz` always returns `ok` and never checks Redis/core. K8s can't distinguish "alive" from "ready." Add `/readyz` that pings dependencies.
- 🟢 **Error responses are inconsistent.** Config API uses `{ok:false,error}`; dashboard actions use `{ok:false,error}` with 422; `/` and `/version` use ad-hoc shapes; `writeErr` returns 400 for essentially all core failures (a backend outage reports as client error). Standardize an error envelope + correct status codes.
- 🟢 **`otelResponseRecorder` / `responseRecorder` don't implement `http.Flusher`/`Hijacker`** — fine today (no streaming), but will silently break SSE/WebSocket/flush if added later.

---

## CLI Review (strongest area)

Solid: clear cobra command tree, `runfabricw` guards control-plane commands with a helpful error, sensible flags/defaults, telemetry init failure is non-fatal (warns to stderr), exit code 1 on error.

- 🟠 `runDaemonRestart` uses `time.Sleep(1*time.Second)` (`internal/cli/daemoncmd/daemon.go:308`) to wait for the old process — a race; poll the port/PID instead.
- 🟢 PID-file liveness uses `os.FindProcess`+`Signal(0)` (correct on Unix) but PID reuse can false-positive; consider writing a start-time or using the socket as source of truth.
- 🟢 The admin dashboard (`internal/cli/admin/dashboard.go:296`) binds `:port` (all interfaces) with **no auth and no timeouts**, exposing deployment data/outputs. Bind loopback by default; it's a local tool.

---

## Database / State Review

- No SQL database in core paths — state is JSON files (`.runfabric/`) with optional S3/DynamoDB deploy-state backends. SQL-injection N/A in core; SQLite/DynamoDB state extensions exist under `extensions/states/`.
- 🟠 **Two parallel locking implementations:** `platform/state/locking/file_lock.go` (`0644`, no expiry → **permanent deadlock on crash**) vs `platform/state/locking/file_backend.go` (`0600`, owner-token, stale expiry). Delete the weaker `FileLock`; it is a footgun.
- 🟠 **Lock TOCTOU on stale-lock cleanup** in `file_backend.go` `Acquire` (read → remove expired → `O_EXCL` create). The `O_EXCL` create wins the create race, but the read-check is not atomic; acceptable on local FS, unsafe on network FS / across hosts.
- 🟢 Repeated full `LoadWorkflowRun`/`SaveWorkflowRun` per step/attempt — disk + JSON churn per transition. Fine at small scale, wasteful at target scale.

---

## Performance Review

- Local-FS read-modify-write of whole run JSON per step is the main hotspot at scale.
- Registry install reads entire artifact into memory (`os.ReadFile`, up to 512 MiB cap) for checksum/signature — acceptable for a CLI, not for a server path.
- No connection pooling concerns in core (stateless HTTP client per call; registry client sets a 30–60s timeout — good).
- Redis cache reduces repeat `validate/plan` cost but introduces the secret-caching and OOM risks noted above.

---

## Reliability Review

- ❌ Graceful shutdown — missing (blocker above).
- ❌ Server timeouts — missing (blocker above).
- ⚠️ Retries — present but ctx-ignoring, constant backoff (above).
- ❌ Circuit breakers / fallback — none for cloud/registry calls.
- ⚠️ Health checks — liveness only; no readiness.
- ✅ Crash reconciliation of workflow run status is well thought out.

---

## Observability Review

- ✅ OpenTelemetry tracing wired into the daemon (per-request spans, status on ≥400).
- ❌ **No metrics** (no Prometheus / RED / latency / error counters / saturation).
- ❌ **No structured logging** (daemon child logs are raw stdout→file).
- ⚠️ Audit trail (`mcpPolicy` / `mcpCalls`) is embedded in run JSON rather than a queryable log/stream.
- ⚠️ For 100k users you are effectively blind on throughput/error rates/saturation.

---

## DevOps Review

- ✅ Dockerfile is multistage, non-root (UID 1000), `-trimpath`, has a `HEALTHCHECK`.
- ❌ Broken default args (blocker #1).
- ⚠️ `docker-compose.daemon.yml` publishes Redis `6379:6379` to the host with no password.
- ❌ **No Kubernetes manifests** (no Deployment/Service/PDB/probes/resource limits) despite K8s being a stated target.
- 🟢 `go.mod` declares `go 1.25.0`; local toolchain is `go1.26.0` — pin a `toolchain` directive for reproducible CI builds.

---

## Architecture & Code Quality (condensed)

**Architecture (good bones):** clean `platform/` domain separation, contracts/codec boundaries, pluggable extensions, `go.work` multi-module. Concerns: `platform/workflow` is a ~13k-LOC near-monolith; two parallel locking implementations (above); workflow state coupled to local FS.

**Code quality:** generally idiomatic, good doc comments, errors wrapped with `%w`, atomic-write done right. Duplication: secret-resolution and lock logic exist in more than one place; the inline HTML-string dashboard (`daemon.go:106-163`) mixes presentation into the command.

- 🟢 **Stored-XSS gap:** in the dashboard, `Outputs`/`DeploymentID` are interpolated with `Fprintf` **without `html.EscapeString`**, unlike `App`/`Org`. If deploy outputs are attacker-influenceable, this is reflected/stored XSS.

---

## Production Readiness Scores

| Category | Score | Rationale |
|---|---|---|
| Architecture | **7/10** | Clean domains & contracts; let down by local-FS coupling, dual locks, workflow monolith |
| Code Quality | **7/10** | Idiomatic, well-documented, atomic writes; some duplication & an unescaped HTML path |
| Security | **5/10** | Good defaults (loopback, perms, sig/checksum) undercut by cached secrets, non-constant-time key, single static key, weak SSRF/trust-root |
| Scalability | **2/10** | Local-FS state, no distributed lock/CAS, broken+per-instance rate limit, no shared backend for runs |
| Reliability | **3/10** | No graceful shutdown, no server timeouts, ctx-ignoring retries, no readiness checks |
| Observability | **4/10** | Tracing yes; metrics/structured logs/queryable audit no |
| **Production Readiness** | **3/10** | Not deployable to the stated target without the blockers above |

---

## Top 20 Issues To Fix First

1. Docker default args make the image crash on start — fix entrypoint/key injection.
2. Add graceful shutdown (SIGTERM → `Server.Shutdown`) to daemon, config-api, socket.
3. Add `Read/Write/Idle/ReadHeader` timeouts to every `http.Server`.
4. Workflow run state: add distributed lock or optimistic-concurrency (version/CAS).
5. Move workflow run state + locking to a shared backend (DynamoDB/S3); demote local FS to dev.
6. Stop caching `/resolve` (cleartext secrets in Redis).
7. Constant-time API-key compare.
8. Bound request bodies (`MaxBytesReader` + `LimitReader` in cache middleware).
9. Fix rate limiter: key by IP, evict idle buckets, make it cross-instance (Redis).
10. Retry loops must honor `ctx`; switch to exponential backoff + jitter + hard cap.
11. Replace single static API key with per-principal auth + rotation + audit of caller.
12. Delete the weaker `FileLock` (0644, no expiry → crash deadlock).
13. Add `/readyz` that checks dependencies; keep `/healthz` as liveness.
14. Tighten SSRF allowlist (block metadata/private ranges) and fix the misleading comment.
15. Establish a real artifact trust root; require signatures for non-local installs.
16. HTML-escape deploy `Outputs`/`DeploymentID` in the dashboard (stored-XSS).
17. Add idempotency keys to `/deploy` and `/remove`.
18. Add API versioning (`/v1`) to the daemon API.
19. Add metrics (RED) + structured logging.
20. Add K8s manifests with probes, resource limits, PDB; CI smoke-test the image.

## Quick Wins (<1 day each)

Constant-time compare (#7); server timeouts (#3); `MaxBytesReader` (#8); rate-limiter IP keying + eviction (#9); HTML-escape outputs (#16); delete `FileLock` (#12); `/readyz` (#13); fix Docker entrypoint (#1); OTLP TLS when https; SSRF comment + private-range block (#14).

## Technical Debt

Dual locking implementations; ~13k-LOC workflow package; inline HTML templating in a command; ad-hoc per-endpoint response shapes; audit data embedded in run JSON; `restart` sleep-based race; secret-resolution logic spread across packages.

## Scalability Bottlenecks

Local-FS state for runs + locks (no cross-pod coordination); per-instance in-memory rate limiter; repeated full Load/Save of run JSON per step/attempt (disk+JSON churn); no shared queue → no real horizontal worker model; Redis cache keyed without identity (cross-workspace collision risk).

## Security Risks

Cleartext secrets cached in Redis; non-constant-time key; single shared static key (no authz/rotation/tenancy); SSRF to internal HTTPS; weak supply-chain trust (TOFU/local-dev key only); unauthenticated all-interfaces dashboard; unescaped HTML output; OTLP plaintext.

## Refactoring Roadmap

- **Phase 1 (launch-blocking):** graceful shutdown + timeouts + body limits; fix Docker; constant-time key; stop caching `/resolve`; readiness probe.
- **Phase 2 (scale-out):** pluggable shared state+lock backend for workflow runs with CAS; Redis-backed rate limiting; proper auth (OIDC/JWT, scopes, audit).
- **Phase 3 (productionization):** metrics + structured logs; API versioning + idempotency; supply-chain trust root + mandatory signatures; K8s manifests + load testing; collapse the workflow monolith and the duplicate locks.

## Production Launch Blockers (must-fix gate)

Broken Docker default (#1); no graceful shutdown (#2); no timeouts/body limits (#3, #8); no concurrency control on run state (#4) + local-only state (#5); secrets cached in Redis (#6).

**Until these are closed, do not run more than one instance and do not expose it beyond loopback.**
