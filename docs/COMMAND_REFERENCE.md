# Command Reference

Global flags (apply when supported): `-c`/`--config` (path to runfabric.yml), `-s`/`--stage` (stage name), `--json` (machine-readable output), `--non-interactive` (disable prompts; for CI/MCP), `-y`/`--yes` (assume yes for any confirmation), `--auto-install-extensions` (prompt to auto-install missing external plugins from registry; combine with `-y` for CI).

## Quick navigation

- **Get started / scaffold**: Project setup and scaffolding
- **Deploy loop**: Core lifecycle
- **Local dev**: Local development and debugging
- **Ops**: Invocation/logs/observability
- **Extensions**: Addons and plugins
- **Auth**: Auth and identity
- **Daemon/UI**: Daemon, dashboard, and config API
- **State/recovery**: State + Recovery and inspection

## Binary profiles (`runfabric` + `runfabricd` + `runfabricw`)

| Binary       | Profile         | Command surface |
| ------------ | --------------- | --------------- |
| `runfabric`  | Control-plane   | Full CLI: project/init, doctor/plan/build/package/deploy/remove, invoke/logs/traces/metrics, state, extensions, router, admin, workflow |
| `runfabricd` | Daemon-plane    | Daemon-focused surface: run/start/stop/restart/status for long-running config API + dashboard process |
| `runfabricw` | Workload-plane  | Workflow runtime only: `workflow run`, `workflow status`, `workflow cancel`, `workflow replay` |

Notes:

- `runfabricd` is the only binary that executes daemon lifecycle commands (`runfabricd`, `runfabricd start|stop|restart|status`).
- `runfabricw` intentionally does **not** expose deploy/remove/state/admin/router commands.
- If a control-plane command is run through `runfabricw`, the CLI returns an actionable error that points to `runfabric`.
- Global flags still apply to workflow commands in both binaries (`-c`, `-s`, `--json`, etc.).

## CLI organization (developer note)

- Commands are grouped by domain under `internal/cli/` (`lifecycle`, `invocation`, `project`, `configuration`, `extensions`, `infrastructure`, `admin`, `router`, `common`) and registered in root.
- Binary roots are split by code boundary: control-plane root in `internal/cli`, daemon root in `internal/cli/daemon`, workload-plane root in `internal/cli/worker`.
- CLI command handlers call canonical workflow orchestration packages under `platform/workflow/app`.
- Preferred grouped namespaces: `runfabric auth ...`, `runfabric invoke ...`, `runfabric state ...`, `runfabric extensions ...`.

## Project setup and scaffolding

- `runfabric init [--dir <path>] [--template <http|queue|cron|storage|eventbridge|pubsub>] [--provider <name>] [--state-backend <local|postgres|s3|gcs|azblob>] [--lang <node|ts|js|python|go>] [--service <name>] [--pm <npm|pnpm|yarn|bun>] [--skip-install] [--call-local] [--with-build] [--with-ci <github-actions>] [--no-interactive]` — Use `--with-ci github-actions` to add `.github/workflows/deploy.yml` (doctor → plan → deploy on push). `init` currently scaffolds a subset of backends for simplicity.
- `runfabric generate` — Scaffold artifacts in an existing project: `generate function`, `generate worker`, `generate resource`, `generate addon`, `generate provider-override`, `generate plugin`.
- `runfabric generate function <name> [--trigger http|cron|queue] [--route <method>:<path>] [--schedule <cron>] [--queue-name <name>] [--provider <key>] [--lang js|ts|python|go] [--entry <path>] [--dry-run] [--force] [--no-backup] [--interactive|--no-interactive] [--json]` — Add a new function: creates handler file and patches runfabric.yml. Infers provider and language from config and project; use `--trigger` (default http), `--route` (e.g. GET:/hello), `--schedule`, `--queue-name`. Fails if function name already exists; `--force` overwrites handler file only.
- `runfabric generate worker <name> [--trigger queue] [--queue-name <name>] [--provider <key>] [--lang js|ts|python|go] [--entry <path>] [--dry-run] [--force] [--no-backup] [--interactive|--no-interactive] [--json]` — Add a queue worker function. Equivalent to `runfabric generate function <name> --trigger queue` with queue trigger as the default.
- `runfabric generate resource <name> [--type database|cache|queue] [--connection-env <VAR>] [--dry-run] [--no-backup] [--interactive|--no-interactive] [--json]` — Add `resources.<name>` to runfabric.yml (type and connection env var for DATABASE_URL, REDIS_URL, etc.).
- `runfabric generate addon <name> [--version <semver>] [--dry-run] [--no-backup] [--interactive|--no-interactive] [--json]` — Add `addons.<name>` to runfabric.yml; attach to functions via the function entry's `addons: [<name>]` field.
- `runfabric generate provider-override <key> [--provider <name>] [--runtime <runtime>] [--region <region>] [--dry-run] [--no-backup] [--interactive|--no-interactive] [--json]` — Add `providerOverrides.<key>` for multi-cloud; use with `runfabric deploy --provider <key>`.
- `runfabric generate plugin <provider-id> [--dir <path>] [--module <go-module>] [--version <semver>] [--plugin-version <contract>] [--with-observability] [--dry-run] [--force] [--interactive|--no-interactive]` — Generate standalone Go provider plugin boilerplate (plugin.yaml, go.mod, main.go, README) compatible with external provider adapter method names (`Handshake`, `Doctor`, `Plan`, `Deploy`, `Remove`, `Invoke`, `Logs`). `--with-observability` adds `FetchMetrics` and `FetchTraces` stubs plus advertised capability metadata.

Generate interactive notes:

- `--interactive` prompts for missing/invalid values, previews changes, and asks for confirmation before file writes.
- `--no-interactive` (or global `--non-interactive`) disables prompts for automation and CI.
- Using both `--interactive` and `--no-interactive` is invalid.

## Docs and workflow utilities

- `runfabric docs check [--config <path>] [--stage <name>] [--readme <path>] [--json]`
- `runfabric docs sync [--config <path>] [--stage <name>] [--readme <path>] [--dry-run] [--json]`
- `runfabric workflow run -c <config> [--stage <name>] [--provider <key>] [--name <workflow-name>] [--run-id <id>] [--input <json-object>] [--json]` — Start a workflow run from `workflows`.
- `runfabric workflow status -c <config> [--stage <name>] --run-id <id> [--json]` — Read a workflow run record from local run state.
- `runfabric workflow cancel -c <config> [--stage <name>] --run-id <id> [--json]` — Mark a workflow run as cancel-requested.
- `runfabric workflow replay -c <config> [--stage <name>] [--provider <key>] --run-id <id> --from-step <step-id> [--json]` — Replay a run from a specific step.

Worker-binary equivalents:

- `runfabricw workflow run|status|cancel|replay ...` — same workflow runtime operations through the workload-plane binary.

Workflow runtime behavior notes:

- Workflow commands execute through one in-process durable runtime loop (`WorkflowRuntime`) that combines scheduling + dispatch responsibilities for step execution.
- Pause/resume semantics are durable: `human-approval` steps pause the run until approval input is persisted, then execution continues on resume/replay.
- `workflow cancel` is boundary-based cancellation: it sets cancel-requested and the runtime applies cancellation on the next safe transition boundary.
- Provider-native orchestration bindings (AWS Step Functions / GCP Cloud Workflows / Azure Durable) are extension/provider orchestration concerns and are separate from the local workflow runtime loop.

## Core lifecycle (doctor → plan → build/package → deploy → operate → remove)

- `runfabric doctor -c <config> [--stage <name>]`
- `runfabric plan -c <config> [--stage <name>] [--provider <key>] [--json]` — Use `--provider` when `runfabric.yml` has `providerOverrides` (multi-cloud); e.g. `--provider aws`.
- `runfabric build -c <config> [--stage <name>] [--out <dir>] [--no-cache] [--json]` — Build with per-function content hash cache (`.runfabric/cache`); cache hit skips work; `--no-cache` forces rebuild and stores new hash.
- `runfabric package -c <config> [--stage <name>] [--function <name>] [--out <dir>] [--json]`
- `runfabric deploy -c <config> [--stage <name>] [--function <name>] [--out <dir>] [--preview <id>] [--source <url>] [--provider <key>] [--rollback-on-failure|--no-rollback-on-failure] [--json]` — `--preview pr-123` uses stage `pr-123` for preview environments; `--source <url>` fetches the archive and deploys from temp dir (use `-c <path>` to supply `runfabric.yml` from outside the source); `--provider <key>` selects a provider from `providerOverrides` in `runfabric.yml` (multi-cloud; e.g. `--provider aws --stage prod`).
- `runfabric remove -c <config> [--stage <name>] [--provider <name>] [--json]`

## Deploy history, inspection, and migration

- `runfabric deploy list -c <config> [--json]` — List deployment history (stages and timestamps) from the receipt backend.
- `runfabric releases -c <config> [--json]` — Same as `runfabric deploy list`.
- `runfabric list -c <config> [--stage <name>] [--json]` — List functions from runfabric.yml and deployment status (from receipt).
- `runfabric inspect -c <config> [--stage <name>] [--json]` — Show lock, journal, and receipt state for the current backend.

## Local development and debugging

- `runfabric invoke local -c <config> [--serve] [--watch] [--host <host>] [--port <number>] [--provider <name>] [--method <GET|POST|...>] [--path </route>] [--query <k=v&k2=v2>] [--header <k:v>] [--body <text>] [--event <file>] [--entry <path>]` — With `--serve --watch`, polls project files (runfabric.yml, _.js, _.ts) and reloads the local server on change. Same behavior as `runfabric invoke dev --watch`.
- `runfabric invoke dev -c <config> [--watch] [--stage <name>] [--provider <name>] [--stream-from <stage>] [--tunnel-url <url>] [--doctor-first] [--preset ...] [--host <host>] [--port <number>] ...` — `--watch`: poll project files (runfabric.yml, _.js, _.ts, etc.) and auto-reload the dev server on change. `--stream-from` / `--tunnel-url`: run local server and run provider live-stream hooks; restores provider state on exit (Ctrl+C) when applicable. `--doctor-first`: run doctor preflight before starting local dev. See [DEV_LIVE_STREAM.md](DEV_LIVE_STREAM.md).
- `runfabric debug -c <config> [--stage <name>] [--host <addr>] [--port <number>] [--json]` — Start local server and print PID for attaching a debugger (default host 127.0.0.1, port 3000).
- `runfabric test -c <config> [--json]` — Run project test suite (npm test, go test, or pytest in project directory).

## Invocation, logs, and observability

- `runfabric invoke run -c <config> [--stage <name>] [--provider <key>] --function <name> [--payload <text-or-json>] [--json]` — Use `--provider` when `runfabric.yml` has `providerOverrides` (multi-cloud). Orchestration targets are invoked with prefixes: `sfn:<name>` (AWS), `cwf:<name>` (GCP), and `durable:<name>` (Azure).
- `runfabric invoke logs -c <config> [--stage <name>] [--provider <key>] (--function <name> | --all) [--service <name>] [--json]` — Unified source: provider logs (AWS: CloudWatch; GCP: Cloud Logging, last 1h; Cloudflare: `wrangler tail` sample with Cloudflare API tail fallback) plus optional local files from `logs.path` in config (default `.runfabric/logs`: `<stage>.log`, `<function>_<stage>.log`). `--all` aggregates by service/stage; `--provider` for multi-cloud. When `--service` is provided, it must match `service` in `runfabric.yml`.
- `runfabric invoke traces [--config <path>] [--stage <name>] [--provider <key>] [--all] [--service <name>] ... [--json]` — `--provider` from `providerOverrides` (multi-cloud). `--all` requests aggregation by service/stage. AWS: X-Ray trace summaries (last 1h). GCP: Cloud Trace summaries (when available). Azure: Application Insights traces (when available). When `--service` is provided, it must match `service` in `runfabric.yml`.
- `runfabric invoke metrics [--config <path>] [--stage <name>] [--provider <key>] [--all] [--service <name>] [--since <iso>] [--json]` — `--provider` from `providerOverrides` (multi-cloud). `--all` requests aggregation by service/stage. AWS Lambda: CloudWatch (Invocations, Errors, Duration, last 1h). GCP: Cloud Monitoring metrics (when available). Azure: Application Insights metrics (when available). When `--service` is provided, it must match `service` in `runfabric.yml`.

## Extensions (addons and plugins)

- `runfabric extensions addons list [--json]` — **v1 catalog view** for RunFabric Addons (Phase 15). List add-ons in the built-in catalog; if `addonCatalogUrl` is set in config, fetches and merges from that URL. Add-ons are declared under `addons`; use a function entry's `addons: [sentry]` field to attach only selected addons per function; secrets are bound at deploy.
- `runfabric extensions addons validate [addon-id] [--json]` — Validate runfabric.yml and optional addon (e.g. `sentry`). If addon-id is given, ensures it is declared in addons or functions.\*.addons.
- `runfabric extensions addons add <addon-id> --function <name> [--json]` — Add an addon to a function (patches the matching function entry's `addons` list in runfabric.yml).
- `runfabric extensions addons remove <addon-id> --function <name> [--json]` — Remove an addon from a function's `addons` list in runfabric.yml. No-op if the addon is not attached to that function.
- `runfabric extensions plugin list [--json]` — List provider plugins (aws, aws-lambda, gcp-functions).
- `runfabric extensions plugin info <name> [--json]` — Show plugin manifest or metadata for a provider.
- `runfabric extensions plugin doctor [name] [-c <config>] [--stage <name>] [--json]` — Run doctor for a provider. If name is omitted, uses provider from config.
- `runfabric extensions plugin enable <name>` — Mark a plugin as enabled (writes to `.runfabric/plugins.json`).
- `runfabric extensions plugin disable <name>` — Mark a plugin as disabled (writes to `.runfabric/plugins.json`).
- `runfabric extensions plugin capabilities <name> [--json]` — Show plugin capabilities (runtimes, triggers, resources).
- `runfabric extensions extension list [--kind provider|runtime|simulator] [--prefer-external] [--show-invalid] [--json]` — List RunFabric Plugins (providers, runtimes, simulators). Merges built-in + external plugins found under `RUNFABRIC_HOME/plugins/...` (default `~/.runfabric/plugins`). `--prefer-external` lets external manifests override built-ins on ID conflict; otherwise built-in wins. `--show-invalid` prints (or returns JSON) for skipped plugin entries and why.
- `runfabric extensions extension info <id> [--version <v>] [--prefer-external] [--json]` — Show plugin manifest for a given plugin ID (e.g. `aws-lambda`, `nodejs`). For external plugins, `--version` selects a specific installed version dir (best-effort).
- `runfabric extensions extension search [query] [--json]` — Search plugins by id or name (case-insensitive).
- `runfabric extensions extension install <id> [--source <url|path>] [--registry <url>] [--registry-token <token>] [--kind provider|runtime|simulator] [--version <v>] [--json]` — Install an external plugin. If `--source` is provided, installs from a `.zip`/`.tar.gz` archive (URL or local file) and validates `plugin.yaml`. If `--source` is omitted, installs via the registry **resolve** endpoint (default registry: `https://registry.runfabric.cloud`, override via `--registry`, `RUNFABRIC_REGISTRY_URL`, or `.runfabricrc` `registry.url`). Auth can be provided via `--registry-token`, `RUNFABRIC_REGISTRY_TOKEN`, or `.runfabricrc` `registry.token`.
- `runfabric extensions extension uninstall <id> [--kind provider|runtime|simulator] [--version <v>] [--json]` — Uninstall an installed external plugin (remove one version or all versions).
- `runfabric extensions extension upgrade <id> [--source <url|path>] [--registry <url>] [--registry-token <token>] [--kind provider|runtime|simulator] [--json]` — Upgrade an external plugin by reinstalling it from the given source (archive or registry resolve). Registry config/auth can also come from `.runfabricrc`.
- `runfabric extensions publish init <id> --version <v> --artifact <path> [--type plugin|addon] [--plugin-kind provider|runtime|simulator] [--registry <url>] [--registry-token <token>] [--json]` — Create a publish session and receive signed upload URLs. Saves local session metadata under `RUNFABRIC_HOME/publish-sessions/<publishId>.json`.
- `runfabric extensions publish upload --publish-id <id> [--key <file-key>] [--artifact <path>] [--json]` — Upload staged files for a publish session using saved upload URLs.
- `runfabric extensions publish finalize --publish-id <id> [--registry <url>] [--registry-token <token>] [--json]` — Finalize publish after uploads complete.
- `runfabric extensions publish status --publish-id <id> [--registry <url>] [--registry-token <token>] [--json]` — Check publish session status (`staged`, `uploaded`, `published`, etc.).
- `runfabric extensions providers` — Provider extension utility command.
- `runfabric extensions primitives [--kind triggers|resources|workflows|all] [--provider <id>] [--json]` — Discover supported primitives: trigger capability matrix, managed resource primitives, and workflow step primitives.

## Auth and identity

- `runfabric auth login [--auth-url <url>] [--client-id <id>] [--scope <scopes>] [--json]` — Start OAuth device-code login: request device code, show verification URL/user code, poll token endpoint, store local tokens, and set active session.
- `runfabric auth whoami [--auth-url <url>] [--json]` — Call `/me` with active token and print current identity.
- `runfabric auth logout [--auth-url <url>] [--remote] [--json]` — Delete local tokens and optionally call `/auth/logout`.
- `runfabric auth token list [--json]` — List local token/session metadata (redacted; no raw token values).
- `runfabric auth token revoke [<token-id>] [--all] [--auth-url <url>] [--json]` — Revoke selected token(s) via auth API and remove local token records.

Auth URL resolution order: `--auth-url` -> `RUNFABRIC_AUTH_URL` -> `.runfabricrc` `auth.url` -> `.runfabricrc` `registry.url` -> `https://auth.runfabric.cloud`.

## Daemon, dashboard, and config API

- `runfabric config-api [--address <addr>] [--port <number>] [--stage <name>] [--api-key <key>] [--rate-limit <n>]` — Run the Configuration API server (default `0.0.0.0:8765`). Endpoints: **POST /validate**, **POST /resolve** (body: YAML, query `stage=dev`), **POST /plan**, **POST /deploy**, **POST /remove**, **POST /releases** (body: YAML, query `stage=dev`). Optional `--api-key` requires `X-API-Key` header; `--rate-limit` limits requests per minute per client. For CI and dashboards.
- `runfabric dashboard [--config <path>] [--stage <name>] [--port <number>]` — Start a local web UI (default port 3000) showing project name, stage selector, last deploy status, and **Plan**, **Deploy**, **Remove** action buttons that run against the current config and stage. Use `?stage=<name>` in the browser to switch stage.
- `runfabricd [options]` — Long-running process: config API (POST /validate, /resolve, /plan, /deploy, /remove, /releases) and optionally dashboard at GET / when `--dashboard` is set (requires `--config`). Default port 8766. Runs in foreground.
- `runfabricd start [options]` — Start the daemon in the background; PID in `.runfabric/daemon.pid`, logs in `.runfabric/daemon.log`. Run from project root.
- `runfabricd stop` — Stop the daemon started with `runfabricd start` (reads PID from `.runfabric/daemon.pid`).
- `runfabricd restart` — Stop the daemon (if running) and start it again in the background.
- `runfabricd status` — Report whether the daemon is running (reads .runfabric/daemon.pid and checks process). See [DAEMON.md](DAEMON.md).

`invoke` and `logs` resolve project context from the current working directory.

## Recovery and inspection

- `runfabric recover -c <config> [--stage <name>] [--mode rollback|resume|inspect] [--json]` — Recover from an unfinished transaction journal. Default mode: rollback.
- `runfabric recover-dry-run -c <config> [--stage <name>] [--json]` — Inspect recovery feasibility without mutating state.
- `runfabric state unlock -c <config> [--stage <name>] [--force] [--json]` — Release the deploy lock (use with care).
- `runfabric state lock-steal -c <config> [--stage <name>] [--json]` — Steal the deploy lock (e.g. after a crashed process).
- `runfabric state backend-migrate -c <config> [--stage <name>] [--target <local|postgres|sqlite|s3|aws|dynamodb|gcs|azblob>] [--json]` — Migrate receipt and journal to another backend.

## Runtime router

- `runfabric router deploy [--rollback-on-failure|--no-rollback-on-failure] [--sync-dns] [--sync-dns-dry-run] [--allow-prod-dns-sync] [--enforce-dns-sync-stage-rollout] [--zone-id <id>] [--account-id <id>] [--json]` — Active-active deploy to all `fabric.targets` (provider keys from runfabric.yml). Saves endpoints to `.runfabric/fabric-<stage>.json`. Requires `fabric` and `providerOverrides` in config.
  Optional post-deploy hook: `--sync-dns` runs the same reconciliation flow as `router dns-sync` immediately after a successful deploy.
  Config-driven policy mode: `extensions.router.autoApply` can trigger post-deploy DNS sync automatically (without `--sync-dns`) for selected stages.
  With `--enforce-dns-sync-stage-rollout`, staged policy checks are enforced:
  - `dev`: allowed by default
  - `staging`: requires `RUNFABRIC_DNS_SYNC_DEV_APPROVED=true`
  - `prod`: requires `RUNFABRIC_DNS_SYNC_STAGING_APPROVED=true` and `--allow-prod-dns-sync`
- `runfabric router status [--json]` — HTTP GET each router endpoint and report healthy/fail.
- `runfabric router endpoints [--json]` — List router endpoint URLs (e.g. for Route53 failover or latency routing).
- `runfabric router routing [--json]` — Generate DNS/load-balancer routing hints from `fabric.routing` and deployed router endpoints.
  JSON output uses deterministic contract `runfabric.fabric.routing.v1` with top-level fields: `contract`, `service`, `stage`, `hostname`, `strategy`, `healthPath`, `ttl`, `endpoints`, plus optional `dns` and `loadBalancer` hints.
- `runfabric router simulate [--requests <n>] [--down <provider>]... [--json]` — Local routing simulation with synthetic request distribution (no provider API calls). Useful for weighted steering and failure rehearsal.
- `runfabric router chaos-verify [--requests <n>] [--json]` — Automated failover verification: runs one-endpoint-down and all-endpoints-down scenarios and reports pass/fail.
- `runfabric router dns-sync [--dry-run] [--allow-prod-dns-sync] [--enforce-dns-sync-stage-rollout] [--zone-id <id>] [--account-id <id>] [--json]` — Apply the router routing contract to Cloudflare DNS and (optionally) Load Balancer idempotently.
  Reads router API token from `RUNFABRIC_ROUTER_API_TOKEN`. Additional sources: `extensions.router.credentials.apiTokenSecretRef` (resolved via top-level `secrets`) and file-backed secret envs (`RUNFABRIC_ROUTER_API_TOKEN_FILE`). Zone ID and Account ID may be passed via flags or env (`RUNFABRIC_ROUTER_ZONE_ID` / `RUNFABRIC_ROUTER_ACCOUNT_ID`, configurable via `extensions.router.credentials.*Env`).
  - **DNS-only mode** (no Account ID): creates/updates a CNAME record at `hostname` pointing to the primary endpoint.
  - **Full LB mode** (Account ID present): also reconciles an HTTPS health-check monitor, an LB pool (origins = all router endpoints), and a zone-level load balancer with the configured steering policy (`dynamic_latency` | `off` | `random`).
  - Optional policy-as-code preflight (`extensions.router.mutationPolicy`) can require explicit approval for risky or high-volume mutations before apply.
  - Optional short-lived credential attestation policy (`extensions.router.credentialPolicy`) can require attestation and token TTL/remaining lifetime checks before apply.
  - Drift detection: resources are only mutated when content, origin list, steering policy, or TTL differs from the live state. No deletions without an explicit flag (not yet implemented).
  - Use `--dry-run` to preview all planned changes before applying.
- `runfabric router dns-shift --provider <name> [--percent <n>] [--dry-run] [--allow-prod-dns-sync] [--enforce-dns-sync-stage-rollout] [--zone-id <id>] [--account-id <id>] [--json]` — Progressive canary traffic shift. Reweights router endpoints (1-99% target) and applies through DNS/LB sync.
- `runfabric router dns-reconcile [--apply] [--allow-prod-dns-sync] [--enforce-dns-sync-stage-rollout] [--zone-id <id>] [--account-id <id>] [--json]` — Drift-focused UX for DNS sync.
  - Default mode (`--apply` omitted): dry-run drift report with create/update/no-op summary.
  - Includes resource-level breakdown, delete-candidate preview for managed duplicates, and rolling trend summary from sync history snapshots.
  - Apply mode (`--apply`): reconciles desired routing to provider state.
- `runfabric router dns-restore [--snapshot-id <id>|--latest] [--dry-run] [--allow-prod-dns-sync] [--enforce-dns-sync-stage-rollout] [--zone-id <id>] [--account-id <id>] [--json]` — Restore routing from saved sync snapshots.
  - Snapshots are persisted under `.runfabric/router-sync-<stage>.json`.
  - Snapshot records include operation ID, before/after actions, summaries, and structured events for audit/replay.
  - Default restore target is previous applied snapshot (last-known-good before latest apply).
- `runfabric router dns-history [--window <n>] [--json]` — Show router sync history analytics and trend summary from `.runfabric/router-sync-<stage>.json`.

Router backend selection: `extensions.routerPlugin` defaults to `cloudflare`. Built-ins also include `route53`, `ns1`, and `azure-traffic-manager` provider API reconcilers.
CI rollout template: see [ROUTER_CI_TEMPLATE.md](ROUTER_CI_TEMPLATE.md) for a `dev -> staging -> prod` pipeline baseline.
Operator runbook: [ROUTER_OPERATIONS_WORKFLOW.md](ROUTER_OPERATIONS_WORKFLOW.md).

## Compose

- `runfabric compose plan -f runfabric.compose.yml [--stage <name>] [--concurrency <number>] [--json]`
- `runfabric compose deploy -f runfabric.compose.yml [--stage <name>] [--rollback-on-failure|--no-rollback-on-failure] [--concurrency <number>] [--json]`
- `runfabric compose remove -f runfabric.compose.yml [--stage <name>] [--provider <name>] [--concurrency <number>] [--json]`

**Compose concurrency:** Services are deployed in dependency order (DAG). `--concurrency <n>` limits how many services are deployed in parallel (default is implementation-defined; typically 1–4). Use a lower value (e.g. 1) to avoid hammering a single provider when many services target the same cloud.

## State

Supported backend kinds for state commands: `local`, `postgres`, `sqlite`, `s3`, `dynamodb`, `gcs`, `azblob`.

- `runfabric state pull --provider <name> [--config <path>] [--backend <kind>] [--stage <name>] [--service <name>] [--json]`
- `runfabric state list [--config <path>] [--backend <kind>] [--stage <name>] [--service <name>] [--provider <name>] [--json]`
- `runfabric state backup [--config <path>] [--backend <kind>] [--stage <name>] [--service <name>] [--provider <name>] [--out <path>] [--json]`
- `runfabric state restore --file <path> [--config <path>] [--backend <kind>] [--stage <name>] [--json]`
- `runfabric state force-unlock --provider <name> [--config <path>] [--backend <kind>] [--stage <name>] [--service <name>] [--json]`
- `runfabric state migrate --from <backend> --to <backend> [--config <path>] [--backend <kind>] [--stage <name>] [--service <name>] [--provider <name>] [--json]`
- `runfabric state reconcile [--config <path>] [--backend <kind>] [--stage <name>] [--service <name>] [--provider <name>] [--json]`

## Failure and recovery notes

- Deploy exit codes:
  - `0` all success
  - `2` partial failures
  - `1` full failure
- Optional deploy rollback precedence:
  - CLI flag: `--rollback-on-failure` / `--no-rollback-on-failure`
  - config: `deploy.rollbackOnFailure` in `runfabric.yml`
  - env fallback: `RUNFABRIC_ROLLBACK_ON_FAILURE=1`
- Deploy progress/state logs are shown in standard output; use `--json` for machine-readable output without progress logs.
- Remove recovery notes: `.runfabric/recovery/remove/*.json`

Troubleshooting:

- If you see `command "<name>" is not available in runfabricw`, switch to `runfabric` for control-plane operations (deploy/remove/state/router/admin).
- If you see `unknown command "daemon" for "runfabric"`, use `runfabricd` for daemon operations.

**See also:** [RUNFABRIC_YML_REFERENCE.md](RUNFABRIC_YML_REFERENCE.md) for config (including `resources` and per-function `resources: [key, ...]` for DATABASE_URL/REDIS_URL binding, `deploy.rollbackOnFailure`, `deploy.healthCheck`, `deploy.scaling`, `providerOverrides`, `layers`, and `state`). [TESTING_GUIDE.md](TESTING_GUIDE.md) for call-local, invoke, and CI. [MCP.md](MCP.md) for the MCP server (agents/IDEs).
