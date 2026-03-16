# Command Reference

Global flags (apply when supported): `-c`/`--config` (path to runfabric.yml), `-s`/`--stage` (stage name), `--json` (machine-readable output).

## Core

- `runfabric init [--dir <path>] [--template <api|worker|queue|cron|storage|eventbridge|pubsub>] [--provider <name>] [--state-backend <local|postgres|s3|gcs|azblob>] [--lang <node|ts|js|python|go>] [--service <name>] [--pm <npm|pnpm|yarn|bun>] [--skip-install] [--call-local] [--no-interactive]`
- `runfabric docs check [--config <path>] [--stage <name>] [--readme <path>] [--json]`
- `runfabric docs sync [--config <path>] [--stage <name>] [--readme <path>] [--dry-run] [--json]`
- `runfabric doctor -c <config> [--stage <name>]`
- `runfabric plan -c <config> [--stage <name>] [--provider <key>] [--json]` — Use `--provider` when `runfabric.yml` has `providerOverrides` (multi-cloud); e.g. `--provider aws`.
- `runfabric build -c <config> [--stage <name>] [--out <dir>] [--json]`
- `runfabric package -c <config> [--stage <name>] [--function <name>] [--out <dir>] [--json]`
- `runfabric deploy -c <config> [--stage <name>] [--function <name>] [--out <dir>] [--preview <id>] [--source <url>] [--provider <key>] [--rollback-on-failure|--no-rollback-on-failure] [--json]` — `--preview pr-123` uses stage `pr-123` for preview environments; `--source <url>` fetches the archive and deploys from temp dir (use `-c <path>` to supply `runfabric.yml` from outside the source); `--provider <key>` selects a provider from `providerOverrides` in `runfabric.yml` (multi-cloud; e.g. `--provider aws --stage prod`).
- `runfabric deploy list -c <config> [--json]` — List deployment history (stages and timestamps) from the receipt backend.
- `runfabric releases -c <config> [--json]` — Same as `runfabric deploy list`.
- `runfabric deploy fn <name> -c <config> [--stage <name>] [--out <dir>] [--rollback-on-failure|--no-rollback-on-failure] [--json]`
- `runfabric deploy function <name> -c <config> [--stage <name>] [--out <dir>] [--rollback-on-failure|--no-rollback-on-failure] [--json]`
- `runfabric deploy-function <name> -c <config> [--stage <name>] [--out <dir>] [--rollback-on-failure|--no-rollback-on-failure] [--json]`
- `runfabric remove -c <config> [--stage <name>] [--provider <name>] [--json]`
- `runfabric list -c <config> [--stage <name>] [--json]` — List functions from runfabric.yml and deployment status (from receipt).
- `runfabric inspect -c <config> [--stage <name>] [--json]` — Show lock, journal, and receipt state for the current backend.
- `runfabric migrate --input <serverless.yml> [--output <runfabric.yml>] [--provider <id>] [--dry-run] [--force] [--json]`
- `runfabric call-local -c <config> [--serve] [--watch] [--host <host>] [--port <number>] [--provider <name>] [--method <GET|POST|...>] [--path </route>] [--query <k=v&k2=v2>] [--header <k:v>] [--body <text>] [--event <file>] [--entry <path>]`
- `runfabric dev -c <config> [--stage <name>] [--provider <name>] [--stream-from <stage>] [--tunnel-url <url>] [--preset ...] [--host <host>] [--port <number>] ...` — `--stream-from`: run local server for that stage. `--tunnel-url`: your public tunnel URL. When both are set, **AWS:** CLI points API Gateway routes at the tunnel and restores them on exit (Ctrl+C). See [DEV_LIVE_STREAM.md](DEV_LIVE_STREAM.md).
- `runfabric invoke -c <config> [--stage <name>] [--provider <key>] --function <name> [--payload <text-or-json>] [--json]` — Use `--provider` when `runfabric.yml` has `providerOverrides` (multi-cloud).
- `runfabric logs -c <config> [--stage <name>] [--provider <key>] (--function <name> | --all) [--service <name>] [--json]` — Unified source: provider logs (AWS: CloudWatch; GCP: Cloud Logging, last 1h) plus optional local files from `logs.path` in config (default `.runfabric/logs`: `<stage>.log`, `<function>_<stage>.log`). `--all` aggregates by service/stage; `--provider` for multi-cloud.
- `runfabric traces [--config <path>] [--stage <name>] [--provider <key>] [--all] [--service <name>] ... [--json]` — `--provider` from `providerOverrides` (multi-cloud). `--all` requests aggregation by service/stage. AWS: X-Ray trace summaries (last 1h). GCP/Azure: provider stubs.
- `runfabric metrics [--config <path>] [--stage <name>] [--provider <key>] [--all] [--service <name>] [--since <iso>] [--json]` — `--provider` from `providerOverrides` (multi-cloud). `--all` requests aggregation by service/stage. AWS Lambda: CloudWatch (Invocations, Errors, Duration, last 1h). GCP/Azure: provider stubs.
- `runfabric addons list [--json]` — List add-ons in the built-in catalog; if `addonCatalogUrl` is set in config, fetches and merges from that URL. Add-ons are declared under `addons`; use `functions.<name>.addons: [sentry]` to attach only selected addons per function; secrets are bound at deploy.
- `runfabric providers`
- `runfabric primitives`
- `runfabric config-api [--address <addr>] [--port <number>] [--stage <name>] [--api-key <key>] [--rate-limit <n>]` — Run the Configuration API server (default `0.0.0.0:8765`). Endpoints: **POST /validate**, **POST /resolve** (body: YAML, query `stage=dev`), **POST /plan**, **POST /deploy**, **POST /remove**, **POST /releases** (body: YAML, query `stage=dev`). Optional `--api-key` requires `X-API-Key` header; `--rate-limit` limits requests per minute per client. For CI and dashboards.
- `runfabric dashboard [--config <path>] [--stage <name>] [--port <number>]` — Start a local web UI (default port 3000) showing project name, stage selector, last deploy status, and **Plan**, **Deploy**, **Remove** action buttons that run against the current config and stage. Use `?stage=<name>` in the browser to switch stage.
- `runfabric daemon [--address <addr>] [--port <number>] [--config <path>] [--stage <name>] [--api-key <key>] [--rate-limit <n>] [--dashboard] [--workspace <dir>]` — Long-running process: config API (POST /validate, /resolve, /plan, /deploy, /remove, /releases) and optionally dashboard at GET / when `--dashboard` is set (requires `--config`). Default port 8766. See [DAEMON.md](DAEMON.md).
- `runfabric test -c <config> [--json]` — Run project test suite (npm test, go test, or pytest in project directory).
- `runfabric debug -c <config> [--stage <name>] [--host <addr>] [--port <number>] [--json]` — Start local server and print PID for attaching a debugger (default host 127.0.0.1, port 3000).

`invoke` and `logs` resolve project context from the current working directory.

## Recovery and inspection

- `runfabric recover -c <config> [--stage <name>] [--mode rollback|resume|inspect] [--json]` — Recover from an unfinished transaction journal. Default mode: rollback.
- `runfabric recover-dry-run -c <config> [--stage <name>] [--json]` — Inspect recovery feasibility without mutating state.
- `runfabric unlock -c <config> [--stage <name>] [--force] [--json]` — Release the deploy lock (use with care).
- `runfabric lock-steal -c <config> [--stage <name>] [--json]` — Steal the deploy lock (e.g. after a crashed process).
- `runfabric backend-migrate -c <config> [--stage <name>] [--target <local|aws-remote|...>] [--json]` — Migrate receipt and journal to another backend.

## Runtime fabric

- `runfabric fabric deploy [--rollback-on-failure|--no-rollback-on-failure] [--json]` — Active-active deploy to all `fabric.targets` (provider keys from runfabric.yml). Saves endpoints to `.runfabric/fabric-<stage>.json`. Requires `fabric` and `providerOverrides` in config.
- `runfabric fabric status [--json]` — HTTP GET each fabric endpoint and report healthy/fail.
- `runfabric fabric endpoints [--json]` — List fabric endpoint URLs (e.g. for Route53 failover or latency routing).

## Compose

- `runfabric compose plan -f runfabric.compose.yml [--stage <name>] [--concurrency <number>] [--json]`
- `runfabric compose deploy -f runfabric.compose.yml [--stage <name>] [--rollback-on-failure|--no-rollback-on-failure] [--concurrency <number>] [--json]`
- `runfabric compose remove -f runfabric.compose.yml [--stage <name>] [--provider <name>] [--concurrency <number>] [--json]`

## State

- `runfabric state pull --provider <name> [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--service <name>] [--json]`
- `runfabric state list [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--service <name>] [--provider <name>] [--json]`
- `runfabric state backup [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--service <name>] [--provider <name>] [--out <path>] [--json]`
- `runfabric state restore --file <path> [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--json]`
- `runfabric state force-unlock --provider <name> [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--service <name>] [--json]`
- `runfabric state migrate --from <backend> --to <backend> [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--service <name>] [--provider <name>] [--json]`
- `runfabric state reconcile [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--service <name>] [--provider <name>] [--json]`

## Failure and Recovery

- Deploy exit codes:
  - `0` all success
  - `2` partial failures
  - `1` full failure
- Optional deploy rollback precedence:
  - CLI flag: `--rollback-on-failure` / `--no-rollback-on-failure`
  - config: `deploy.rollbackOnFailure` in `runfabric.yml`
  - legacy env fallback: `RUNFABRIC_ROLLBACK_ON_FAILURE=1`
- Deploy progress/state logs are shown in standard output; use `--json` for machine-readable output without progress logs.
- Remove recovery notes: `.runfabric/recovery/remove/*.json`

**See also:** [RUNFABRIC_YML_REFERENCE.md](RUNFABRIC_YML_REFERENCE.md) for config (including `resources` and per-function `resources: [key, ...]` for DATABASE_URL/REDIS_URL binding, `deploy.rollbackOnFailure`, `deploy.healthCheck`, `deploy.scaling`, `providerOverrides`, `layers`, and `state`). [TESTING_GUIDE.md](TESTING_GUIDE.md) for call-local, invoke, and CI. [ROADMAP.md](ROADMAP.md) for current scope and next steps. [MCP.md](MCP.md) for the MCP server (agents/IDEs).
