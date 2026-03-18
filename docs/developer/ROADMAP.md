# RunFabric Roadmap

RunFabric is a **multi-provider serverless framework package**: one config, one CLI workflow across cloud providers, deploying on managed serverless services that auto-scale and keep idle-cost overhead low. This page lists current scope and all roadmap items by phase (priority order).

---

## Quick navigation

- **What exists today**: Current: Core Framework
- **What’s next**: Phase 15 — Extensions
- **Known gaps**: Stubs and missing features

## 1. Current: Core Framework

What exists today:

- **Entry:** `runfabric` CLI and optional SDKs (Node, Python).
- **Config:** `runfabric.yml` — service, provider, functions, triggers, resources (managed binding), addons, layers, providerOverrides, fabric (active-active), deploy (healthCheck, scaling, strategy: all-at-once | canary | blue-green), build (order), alerts (webhook, slack, onError, onTimeout), app/org (optional grouping).
- **Providers:** AWS, GCP, Azure, Cloudflare, Vercel, Netlify, Fly, DigitalOcean, Alibaba, IBM, Kubernetes.
- **Flow:** `doctor` → `plan` → `build` / `package` → `deploy` → `invoke` / `logs` / `traces` / `metrics` → `remove`; `init` (with optional `--with-ci github-actions`); `generate function`; compose (plan/deploy/remove); fabric (deploy/status/endpoints); config-api (validate/resolve); dashboard (local UI); daemon (single process: API + optional dashboard, `--workspace` for project root). Local dev: `call-local` / `dev` with `--watch` for file-based reload. See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for per-provider errors and fixes.
- **State:** Local file or optional backend (e.g. S3 + DynamoDB) for locks and deploy receipts.
- **Quality:** `make release-check` (full gate) and `make check-syntax` (fast CI); CLI tests cover doctor, plan, deploy, remove, compose, fabric, config-api, multi-cloud, scaling/health/layers, observability, dev live stream, addons, resource binding; TESTING_GUIDE covers dev mode and layers.

Deploy routing: AWS uses controlplane and deployrunner; other providers use deploy API and adapters. See [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 2. Phases (priority order)

**Phase completion:** A phase is **done** when (1) items are implemented and documented, (2) relevant tests exist and pass in CI, and (3) new flags/config are in the config or command reference.

**Stubs and missing features:** See the **Stubs and missing features** subsection under Phase 15 for a single list of documented or code-present items that are not yet implemented (e.g. `extension install`, `runfabric primitives`, RUNFABRIC_HOME, schema stubs, workflow replay).

### Completed — Phases 1–9

- **Phases 1–7:** Core lifecycle, developer experience, observability, deploy safety, extensibility, integrations, CLI and documentation.
- **Phase 8 — State:** `runfabric state pull`, `list`, `backup`, `restore`, `force-unlock`, `migrate`, `reconcile` wired to backends (local, S3, DynamoDB). See [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md).
- **Phase 9 — Providers & provisioning:** AWS (controlplane, RDS/ElastiCache provisioning), GCP, Azure, Cloudflare, Vercel, Netlify, Fly, DigitalOcean, Alibaba, IBM, Kubernetes — real deploy/remove/invoke/logs per provider contract.

### Completed — Phases 10–13

- **Phase 10:** `runfabric generate function` (scaffold + configpatch), triggers http/cron/queue, tests and [GENERATE_PROPOSAL.md](GENERATE_PROPOSAL.md).
- **Phase 11:** Examples under `examples/node/`, [FILE_STRUCTURE.md](FILE_STRUCTURE.md) / [LAYOUT.md](LAYOUT.md) aligned with repo.
- **Phase 12:** Version/changelog, `make release-check`, release process docs.
- **Phase 13:** Provider contract test + DEPLOY_PROVIDERS sync; `make check-syntax`; call-local/dev `--watch`; [TROUBLESHOOTING.md](TROUBLESHOOTING.md); compose concurrency and code ownership docs; init `--with-ci github-actions`; real-deploy safety in config reference.

### Phase 13.11 (completed)

Pre-Phase 14 refactors done: unified build/package path, configpatch AddMapEntry, aiflow package, JSON envelope, ResolveConfigAndRoot, receipt metadata, package and docs commands, app build, lifecycle build no-op, removed redundant runtimes/node and runtimes/python stubs.

### Phase 13.12 — Stub removal (completed)

Provider and CLI stubs removed (legacy stub provider + CLI stub command helpers).

### Phase 14 — AI workflows (priority: medium)

Goal: Extend RunFabric to **define, validate, deploy, run, and observe AI workflows** as a **first-class config capability inside `runfabric.yml`**, integrated into the existing lifecycle (no parallel “AI deploy path”).

Design constraints (must hold):

- **Single config:** still `runfabric.yml` (no second file).
- **Single lifecycle:** `doctor → plan → build → deploy → remove` stays primary.
- **Minimal AI utilities only:** allow `runfabric ai validate` and `runfabric ai graph` for domain introspection (not a second lifecycle).

Status: AI workflow config + DAG compilation + lifecycle integration + basic utilities are implemented. Remaining follow-ons are tracked under “Stubs and missing features”.


### Phase 15 — Extensions (addons + plugins) (in progress)

Goal: Evolve RunFabric’s extension model into a **clean, dual-track system**:

- **Addons**: function/app-level augmentation with lifecycle hooks (Node/JS based), env injection, handler wrapping, build patching, and instrumentation.
- **Plugins**: Go-side capability implementations for providers, runtimes, and simulators, routed via a registry/resolver.

Foundations (manifests, registry, contracts, initial CLI surface) are implemented. Remaining work below is forward-looking only.

- [ ] **External extensions (Phase 15b–15d):** Install and load plugins from disk (`~/.runfabric/plugins/`). See [EXTERNAL_EXTENSIONS_PLAN.md](EXTERNAL_EXTENSIONS_PLAN.md).
  - [ ] **15b — Discovery (in-depth TODO):**
    - [ ] Define and document `RUNFABRIC_HOME` semantics and default (`~/.runfabric/`), including Windows/macOS/Linux paths.
    - [ ] Finalize `plugin.yaml` schema (fields, validation rules, examples) and add a schema test fixture.
    - [ ] Implement on-disk scan for `plugins/{providers,runtimes,simulators}/<id>/<version>/plugin.yaml`.
    - [ ] Implement semver selection (latest / pinned / “current” strategy if adopted) with deterministic ordering.
    - [ ] Merge external manifests into built-in manifests for `runfabric extension list|info|search` with source metadata (builtin vs external path/version).
    - [ ] Add unit tests for discovery/merge (invalid manifests, duplicates, missing executable, bad versions).
  - [ ] **15c — Subprocess protocol + adapter (in-depth TODO):**
    - [ ] Define the line-delimited JSON request/response envelope and error format (method, params, id/correlation, error codes).
    - [ ] Implement subprocess lifecycle (spawn, handshake/version, timeout, kill on exit) and per-command reuse strategy.
    - [ ] Implement external provider adapter mapping Doctor/Plan/Deploy/Remove/Invoke/Logs to stdio protocol.
    - [ ] Add an integration test with a stub plugin binary (golden request/response fixtures).
    - [ ] Add observability hooks (debug logging + optional OTEL spans) without leaking secrets.
  - [ ] **15d — Install/uninstall/upgrade (in-depth TODO):**
    - [ ] Define marketplace index format (id→kind/version/url/checksum) and allow `--source url` override.
    - [ ] Implement download cache (`~/.runfabric/cache`) and extraction to `~/.runfabric/plugins/...`.
    - [ ] Verify checksums (plugin.yaml inline or checksums file), plus executable presence/permissions.
    - [ ] Implement `extension uninstall` and `extension upgrade` (including “active version” behavior).
    - [ ] Update docs: COMMAND_REFERENCE, EXTERNAL_EXTENSIONS_PLAN, and a short “External extensions quickstart”.
  - [ ] **15e — Registry / Marketplace (MVP v1) (in-depth TODO):**
    - [ ] **Domains (prod)**
      - [ ] `runfabric.cloud` — single frontend for docs + marketplace UI
      - [ ] `registry.runfabric.cloud` — registry API (resolve/search/publish/auth)
      - [ ] `cdn.runfabric.cloud` — immutable artifacts (downloads)
    - [ ] **Spec + schemas (source of truth)**
      - [ ] Implement the MVP contract in [REGISTRY_API_DB_SCHEMA_MVP_V1.md](REGISTRY_API_DB_SCHEMA_MVP_V1.md).
      - [ ] Keep [EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md](EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md) aligned with CLI behavior.
      - [ ] Keep [REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md](REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md) aligned with production posture.
      - [ ] Expand `schemas/registry/` to cover: resolve, search, publish init/finalize, advisories, and standard errors.
    - [ ] **Registry API implementation** (`registry/`)
      - [ ] `GET /v1/extensions/resolve` (DB-backed, cached, highest compatible published version selection).
      - [ ] `GET /v1/extensions/search` (MVP filters + pagination).
      - [ ] `GET /v1/extensions/{id}`, `/versions`, `/versions/{v}`, `/advisories`.
      - [ ] Enforce the standard error envelope on every endpoint (code/message/details/hint/docsUrl/requestId).
    - [ ] **Publishing**
      - [ ] `POST /v1/extensions/publish/init` (namespace ownership, version uniqueness, signed upload URLs).
      - [ ] `POST /v1/extensions/publish/finalize` (re-check checksums, verify signature, validate payload, scan, publish/reject).
      - [ ] `GET /v1/publish/{publishId}` (status polling).
      - [ ] yank/deprecate endpoints; ensure yanked versions are not returned by resolve.
    - [ ] **Auth + RBAC + audit**
      - [ ] Token scopes (`registry:read|publish|manage|admin`) + role enforcement.
      - [ ] Audit logs (publish attempts, auth failures, key rotation/revocation, admin actions).
      - [ ] Rate limiting (per-IP + per-token) and caching for resolve/search.
    - [ ] **CLI integration**
      - [ ] `runfabric extension install` / `upgrade` via registry resolve with checksum + signature verification before install.
      - [ ] Add `runfabric extension publish` (init/upload/finalize/status) with clear UX.
      - [ ] Support `.runfabricrc` defaults for registry URL + auth; document precedence.
    - [ ] **Web frontend (registry + docs) (MVP v1)**
      - [ ] **Marketplace UI** (`web/extensions/`)
        - [ ] Extension browsing: search + filters (type, pluginKind, trust, publisher).
        - [ ] Extension detail page: version list, compatibility, permissions, changelog, integrity links (SBOM/provenance).
        - [ ] Version detail: artifact matrix by os/arch, checksums/signatures, install instructions.
      - [ ] **Publisher portal**
        - [ ] Publish flow UI: start publish session, upload artifacts (or show signed URL instructions), finalize, show status.
        - [ ] Manage namespaces, signing keys (view, rotate, revoke), and release actions (yank/deprecate).
      - [ ] **Auth (Google + GitHub)**
        - [ ] OAuth login for the web UI (Google + GitHub) and an account model that maps identities → publisher.
        - [ ] Session management (secure cookies) and CSRF protection for write endpoints.
        - [ ] Token issuance for CLI: create/revoke `registry:read` and `registry:publish` tokens from the web UI.
      - [ ] **Docs site (registry + extensions)**
        - [ ] Host a docs frontend that renders `docs/user/` and `docs/developer/` (or publishes to a docs site) with clear audience routing.
        - [ ] Add “Registry API reference” pages generated from `schemas/registry/*` (OpenAPI or static schema rendering).
      - [ ] **Ops + security**
        - [ ] Put web + API behind edge protection; rate limits aligned with `REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md`.
        - [ ] Add basic audit views for publishers (publish events, key usage, token creation).
- [ ] **SDK feature sync (handlers + hooks) (priority: medium):**
  - [ ] **Define a parity contract** for SDKs (what every language must support):
    - [ ] **Handler**: consistent `(event, context) -> response` semantics and a stable context shape (stage, service, requestId/trace, secrets redaction rules).
    - [ ] **HTTP adapters**: one “raw” HTTP adapter + at least one popular framework adapter per language where applicable.
    - [ ] **Lifecycle hooks (developer-facing)**: a consistent way to build handler/hook packages (handlers + addon-style hooks) with clear ordering and error semantics.
    - [ ] **Local dev ergonomics**: “run locally the same way you deploy” (single entrypoint where possible).
  - [ ] **Node SDK parity** (`packages/node/sdk`):
    - [ ] Keep `createHandler()` as the canonical entrypoint and ensure adapters (Express/Fastify/Nest/raw) are consistent and documented.
    - [ ] Add hook development docs/examples for addon hooks and handler wrapping.
  - [ ] **Python SDK parity** (`packages/python/runfabric`):
    - [ ] Ensure FastAPI/Flask/Django/raw ASGI/WSGI mounts match Node semantics (status codes, headers, body encoding, error mapping).
    - [ ] Add a “single handler” convenience wrapper to reduce framework-specific boilerplate (mirror Node DX).
  - [ ] **Go/Java/.NET SDK parity** (`packages/go/sdk`, `packages/java/sdk`, `packages/dotnet/sdk`):
    - [ ] Align handler signatures and HTTP wrapper semantics with Node/Python (headers/body/event normalization).
    - [ ] Provide minimal framework adapters where relevant (or document the recommended integration pattern).
  - [ ] **Exception (Go SDK also includes plugin dev)**
    - [ ] The **Go SDK** also includes the **plugin development interface** (wire types + stdio server loop) so external plugins can be authored without importing the engine.
  - [ ] **Tests + docs**
    - [ ] Add cross-SDK contract tests (golden event → response fixtures) to prevent drift.
    - [ ] Update [SDK_FRAMEWORKS.md](SDK_FRAMEWORKS.md) and add “Hooks development” examples per language.
- [ ] **Go SDK for extension development:** Publish a Go SDK (separate module or `packages/go/plugin-sdk`) so external provider plugins can be built without importing the engine. SDK provides: wire request/response types (JSON-compatible with engine protocol), stdio server loop (read line-delimited JSON, dispatch to user’s provider impl, write responses), optional `plugin.yaml` / `--runfabric-protocol=stdio` handling. See [EXTENSION_DEVELOPMENT_GUIDE.md](EXTENSION_DEVELOPMENT_GUIDE.md) §2.5 and [EXTERNAL_EXTENSIONS_PLAN.md](EXTERNAL_EXTENSIONS_PLAN.md).
- [ ] **Runtime plugin interface:** Formal runtime plugin contract (build/invoke) and registry; today only built-in nodejs/python runtimes (see [EXTENSION_DEVELOPMENT_GUIDE.md](EXTENSION_DEVELOPMENT_GUIDE.md)).
- [ ] **Simulator plugin:** Implement simulator kind (local/test double); manifest kind exists, no loader or execution.

- [ ] **Performance & internal optimizations (extensions plumbing):**
  - [ ] **Reuse builtin registries in long-lived processes** (daemon/config-api) — avoid rebuilding builtin provider/runtime registries per request when safe (no mutable shared state).
  - [ ] **Shorten registry lock hold times under concurrency** — snapshot map entries under lock, build/sort results outside lock for `List()`/`Search()`-style methods.

### Stubs and missing features (from docs/code)

These are documented or present in code but not fully implemented. Tracked here so they can be completed or removed.

- **Extensions marketplace / external plugins (priority: high)**
  - [ ] **Registry-backed publish** — implement `runfabric extension publish` end-to-end (init/upload/finalize/status) per Phase 15e.
  - [ ] **Registry-backed install security** — signature verification policy (official/verified required), clear errors for checksum/signature failures, and safe receipts.
  - [ ] **Config `provider.source` / `provider.version`** — Optional config for "use external plugin" and pin version; described in [EXTERNAL_EXTENSIONS_PLAN.md](EXTERNAL_EXTENSIONS_PLAN.md) for future.

- **CLI placeholders and UX gaps (priority: medium)**
  - [ ] **`runfabric primitives`** — Placeholder command (prints "Executing primitives command"); implement (e.g. list trigger/resource primitives) or remove.
  - [ ] **Dev live stream: GCP auto-wire** — docs note `runfabric dev --stream-from` runs local server but does not auto-wire GCP; implement auto-wire or document the supported providers precisely (see [DEV_LIVE_STREAM.md](DEV_LIVE_STREAM.md)).

- **Lifecycle fallback “stub” behavior (priority: medium)**
  - [ ] **Remove/replace lifecycle stubs** — docs note deploy/invoke/logs may fall back to a “lifecycle stub” when a provider lacks API handlers (see [ARCHITECTURE.md](ARCHITECTURE.md)). Decide and implement the desired behavior:
    - Either: hard error with a clear message (“provider not implemented; install external provider plugin / choose supported provider”)
    - Or: implement a real, documented fallback execution path (not a stub)

- **Schemas and validation completeness (priority: medium)**
  - [ ] **Schema stubs: `schemas/resource.schema.json`** — currently a minimal placeholder; expand to match `resources` model and update references.
  - [ ] **Schema stubs: `schemas/workflow.schema.json`** — currently a minimal placeholder; expand or remove if superseded by `aiWorkflow` schema.

- **Docs/behavior alignment (priority: low)**
  - [ ] **Clarify “simulated/stub deploy” wording** — `DEPLOY_PROVIDERS.md` mentions “simulated or stub mode”. Either document the exact non-real-deploy semantics (what happens when `RUNFABRIC_REAL_DEPLOY` is unset) or remove “stub” wording if it’s misleading.

- **AI workflows follow-ons (priority: low)**
  - [ ] **Phase 14: AI workflow replay** — call-local/dev expose compiled workflow in root JSON; "replay" (re-run workflow from a given node) not yet implemented.

(- Completed doc-alignment items are intentionally omitted; this list tracks only open gaps.)

### Phase 16 — AWS Step Functions (state machines) (future)

Goal: Add first-class support for **AWS Step Functions** as an AWS provider capability inside the extensions ecosystem, so users can deploy and operate **state machines** that orchestrate RunFabric-managed Lambdas.

- [ ] Config + schema: declare Step Functions state machines in `runfabric.yml` (name, definition, role/policies, logging/tracing options) as an extension of the provider plugin.
- [ ] Deploy/remove: create/update/delete state machines; wire in Lambda ARNs produced by RunFabric deploys via the AWS provider plugin.
- [ ] Plan/doctor: validate definitions, permissions, and referenced functions; show diffs in plan output.
- [ ] Invoke/inspect: start executions, surface execution history/links, and attach state machine metadata to receipts.

---

## 3. See also

| Doc                                                      | Description                                                                                |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| [ARCHITECTURE.md](ARCHITECTURE.md)                       | Deploy flow and provider layout.                                                           |
| [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md)             | CLI commands and flags.                                                                    |
| [user/DAEMON.md](../user/DAEMON.md)                                   | Daemon: config API + optional dashboard, systemd/launchd.                                   |
| [RUNFABRIC_YML_REFERENCE.md](RUNFABRIC_YML_REFERENCE.md) | Config reference (resources, addons, layers, providerOverrides, deploy, build, alerts, app/org, state backends). |
| [FILE_STRUCTURE.md](FILE_STRUCTURE.md)                   | Repo file layout and package naming.                                                       |
| [LAYOUT.md](LAYOUT.md)                                   | Repository layout (engine, packages, examples).                                            |
| [EXAMPLES_MATRIX.md](EXAMPLES_MATRIX.md)                 | Provider and trigger support.                                                              |
| [DEV_LIVE_STREAM.md](DEV_LIVE_STREAM.md)                 | Dev live stream (`--stream-from`, `--tunnel-url`).                                         |
| [TESTING_GUIDE.md](TESTING_GUIDE.md)                     | Testing with call-local, invoke, and CI.                                                   |
| [PLUGINS.md](PLUGINS.md)                                 | Lifecycle hooks and plugin API contract.                                                   |
| [ADDONS.md](ADDONS.md)                                   | RunFabric Addons (config, catalog, per-function).                                           |
| [ADDON_CONTRACT.md](ADDON_CONTRACT.md)                   | Addon implementation interface (supports, apply, AddonResult).                              |
| [EXTENSION_DEVELOPMENT_GUIDE.md](EXTENSION_DEVELOPMENT_GUIDE.md) | Addon and extension development guidelines (contract, catalog, registry, testing).        |
| [EXTERNAL_EXTENSIONS_PLAN.md](EXTERNAL_EXTENSIONS_PLAN.md)       | Plan for external plugins on disk (~/.runfabric/plugins/), install, and subprocess protocol. |
| [GENERATE_PROPOSAL.md](GENERATE_PROPOSAL.md)             | P1: `runfabric generate function` (in-project scaffolding).                                |
| [CREDENTIALS.md](CREDENTIALS.md)                         | Credentials and secret resolution.                                                         |
| [TROUBLESHOOTING.md](TROUBLESHOOTING.md)                 | Per-provider errors and fixes.                                                             |
| [TELEMETRY.md](TELEMETRY.md)                             | OpenTelemetry tracing (OTLP).                                                              |
| [MCP.md](MCP.md)                                         | MCP server for agents and IDEs (plan, deploy, doctor, invoke).                             |
