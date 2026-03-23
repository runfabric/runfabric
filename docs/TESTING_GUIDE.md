# Testing Guide

This guide covers how to test RunFabric projects locally and in CI: **call-local**, **invoke**, and **CI patterns**.

---

## Quick navigation

- **Fast local loop**: `invoke local`
- **Smoke test a deployment**: `invoke run`
- **CI without deploying**: Config API
- **Interactive remote-to-local**: Dev mode live stream

## invoke local

Use `runfabric invoke local` to run a function handler locally without deploying. Useful for fast feedback during development.

```bash
# From the project directory (where runfabric.yml lives)
runfabric invoke local --stage dev

# With a specific payload (stdin or file)
echo '{"name":"world"}' | runfabric invoke local --stage dev
```

- **Config:** Ensure `runfabric.yml` has the correct `provider`, `runtime`, and function `handler` (e.g. `index.handler` for Node).
- **Dependencies:** Install runtime dependencies first (e.g. `npm install`, `pip install -r requirements.txt`). The CLI runs the handler in the project directory.
- **Environment:** Set any `env` or `params` from config; secrets can be provided via env vars or a local `.env` (not committed).

Use invoke local in unit or integration tests by invoking the CLI from a test script or by importing the handler and calling it directly in tests.

---

## invoke run (remote)

Use `runfabric invoke run` to call an already-deployed function. Good for smoke tests after deploy and for testing against a real stage.

```bash
runfabric invoke run --stage dev --function api
echo '{"body":"test"}' | runfabric invoke run --stage dev --function api
```

- **Prerequisites:** A successful deploy for the same `--stage` and `--config`. The receipt in `.runfabric/<stage>.json` (or the configured backend) is used to resolve the deployed function.
- **CI:** In CI, run `invoke` after `deploy` to verify the deployment. Use `--json` for machine-readable output and to assert on status.

---

## Config API (CI)

For CI or dashboards, run the config API server and call it to validate or resolve config without deploying:

```bash
# Start the server (default http://0.0.0.0:8765)
runfabric config-api --port 8765

# In another terminal or from CI:
curl -s -X POST http://localhost:8765/validate -d @runfabric.yml
# → { "ok": true } or { "ok": false, "error": "..." }

curl -s -X POST "http://localhost:8765/resolve?stage=prod" -d @runfabric.yml
# → { "ok": true, "stage": "prod", "config": { ... } }
```

Use **POST /validate** to check config before build/deploy; use **POST /resolve** to get the resolved config for a given stage (e.g. for templating or debugging).

---

## CI patterns

### Basic pipeline

1. **Install CLI** — Build from source or use the npm wrapper:
   ```bash
   make build   # from repo root, produces bin/runfabric
   # or: npm install -g @runfabric/cli
   ```
2. **Validate config** — Catch config errors early:
   ```bash
   runfabric doctor --config runfabric.yml --stage dev
   runfabric plan --config runfabric.yml --stage dev
   ```
3. **Build** — Produce artifacts (if your flow uses build):
   ```bash
   runfabric build --config runfabric.yml --stage dev
   ```
4. **Deploy** — Deploy to a CI stage (e.g. `ci` or a branch name):
   ```bash
   runfabric deploy --config runfabric.yml --stage ci
   ```
5. **Smoke test** — Invoke the deployed function:
   ```bash
   runfabric invoke run --config runfabric.yml --stage ci --function api
   ```

### Preview / PR environments

Use `--preview` to deploy to an isolated stage (e.g. per pull request):

```bash
runfabric deploy --config runfabric.yml --preview pr-123
runfabric invoke run --config runfabric.yml --stage pr-123 --function api
```

Clean up when the PR is closed (e.g. in a pipeline step):

```bash
runfabric remove --config runfabric.yml --stage pr-123
```

### Deploy from source (CI artifact or URL)

To deploy from an archive (e.g. GitHub tarball or CI artifact) without cloning the full repo:

```bash
runfabric deploy --config runfabric.yml --stage ci --source https://github.com/org/repo/archive/main.tar.gz
```

The CLI fetches the archive, extracts it to a temp dir, finds `runfabric.yml` inside, and runs deploy from there. Supports `.zip` and `.tar.gz`/`.tgz`.

### Compose (multi-service)

For projects using `runfabric.compose.yml`:

```bash
runfabric compose plan -f runfabric.compose.yml --stage dev
runfabric compose deploy -f runfabric.compose.yml --stage ci
runfabric compose remove -f runfabric.compose.yml --stage ci
```

Deploy runs services in dependency order and injects `SERVICE_*_URL` from prior services’ receipt outputs into dependent services.

---

## Dev mode (live stream)

Use `runfabric invoke dev --stream-from <stage>` to run the local server; with `--tunnel-url` (and AWS), the CLI auto-wires API Gateway to the tunnel and restores on exit. See [DEV_LIVE_STREAM.md](DEV_LIVE_STREAM.md).

- **Testing dev locally:** Start the dev server, then in another terminal run `runfabric invoke run` or send HTTP requests to the tunnel. For non-AWS providers, the local server runs without auto-wire; use provider emulators or point triggers at your tunnel URL manually.
- **CI:** Dev mode is typically used interactively; in CI, use `invoke local` for one-off handler tests and `deploy` + `invoke run` for integration.

## Provider observability checks (Phase 6)

- GCP: verify `runfabric invoke logs`, `runfabric invoke traces`, and `runfabric invoke metrics` with `GCP_ACCESS_TOKEN` plus `GCP_PROJECT`/`GCP_PROJECT_ID`.
- Azure: verify the same commands with `AZURE_ACCESS_TOKEN` and `AZURE_LOG_ANALYTICS_WORKSPACE_ID`.
- Cloudflare: verify `runfabric invoke logs` with `wrangler` installed (tail sample path), then set `RUNFABRIC_CLOUDFLARE_DISABLE_WRANGLER_TAIL=1` to verify API-tail fallback behavior.

---

## Layers

When using top-level `layers` and a function entry's `layers` list:

- **Resolve:** Config is resolved (including `${env:...}` in layer `arn`/`version`) at plan/deploy time. Use `runfabric plan` to verify layer resolution without deploying.
- **Testing:** For projects that use layers, run `runfabric plan -c runfabric.yml --stage dev` in CI to ensure layer refs resolve; AWS deploy will attach the resolved ARNs. Other providers ignore layers for now (see RUNFABRIC_YML_REFERENCE).

---

## Unit testing your handlers

- **Node/TypeScript:** Use your normal test runner (Jest, Vitest, etc.) to import and call the handler export. Mock any external services or env vars.
- **Python:** Use pytest (or similar) to import the handler and call it with fixture payloads.
- **Go:** Use `testing` to call the handler function directly.

Keep handler logic pure where possible (payload in, response out) so it’s easy to test without the CLI.

---

## See also

- [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md) — All CLI commands and flags.
- [CONTRIBUTING.md](../CONTRIBUTING.md) — Building and testing the RunFabric repo itself (engine, Makefile, release-check).
