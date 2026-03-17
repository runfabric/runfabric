# RunFabric Daemon

The **daemon** is a long-running HTTP server that exposes the RunFabric Configuration API and an optional web dashboard. Use it for CI, dashboards, or as a shared service that runs plan/deploy/remove without invoking the CLI each time.

## Quick start

```bash
# API only (default port 8766) — runs in foreground (holds terminal)
runfabric daemon

# Start in background (does not hold terminal). Run from project root.
runfabric daemon start
runfabric daemon stop
runfabric daemon restart   # stop (if running) then start
runfabric daemon status    # report if daemon is running (from .runfabric/daemon.pid)

# With dashboard at GET /
runfabric daemon --dashboard --config runfabric.yml

# With API key and rate limit
runfabric daemon --api-key my-secret --rate-limit 60 --dashboard --config runfabric.yml
```

**Background (start/stop):** `runfabric daemon start` spawns the daemon as a detached process. PID is written to `.runfabric/daemon.pid` and logs to `.runfabric/daemon.log`. Run both `start` and `stop` from the same directory (e.g. project root) so they use the same `.runfabric` folder.

## Options

| Flag | Default | Description |
|------|---------|-------------|
| `--address` | `0.0.0.0` | Listen address |
| `--port`, `-p` | `8766` | Listen port (different from config-api default 8765) |
| `--config`, `-c` | `runfabric.yml` | Path to runfabric.yml (needed for `--dashboard` and for API actions that use config) |
| `--stage`, `-s` | `dev` | Default stage for API and dashboard |
| `--dashboard` | `false` | Serve dashboard at GET / (requires `--config`) |
| `--workspace` | (none) | Project root; `--config` is resolved relative to this (e.g. `--workspace .` when run from project root in systemd/launchd) |
| `--cache-ttl` | `5m` | API cache TTL when `--cache-url` is set. Use `0` for per-endpoint defaults (validate 10m, resolve/plan 5m, releases 1m). |
| `--cache-url` | (none) | **Distributed API cache:** Redis URL (e.g. `redis://localhost:6379/0`). When set, caches Config API (POST /validate, /resolve, /plan, /releases). Env: `RUNFABRIC_DAEMON_CACHE_URL`. |
| `--api-key` | (none) | If set, require `X-API-Key` header on all API requests |
| `--rate-limit` | `0` | Max requests per minute per client (0 = disabled) |

## Caching

### Distributed API cache (Redis)

When `--cache-url` is set to a Redis URL, the daemon caches **Config API** responses for **POST /validate**, **POST /resolve**, **POST /plan**, and **POST /releases**. Cache key is `(endpoint, body hash, stage)`. Same YAML body + stage returns the cached response. Deploy and remove are **not** cached; on successful **POST /deploy** or **POST /remove** (or dashboard **POST /action/deploy**, **POST /action/remove**), the cache for that stage is invalidated so the next resolve/plan reflects the new state.

TTL per endpoint: validate 10m, resolve/plan 5m, releases 1m (or `--cache-ttl` if set and shorter).

```bash
runfabric daemon --cache-url redis://localhost:6379/0

# With env
export RUNFABRIC_DAEMON_CACHE_URL=redis://localhost:6379/0
runfabric daemon
```

Supported URL schemes: `redis://` and `rediss://` (TLS). Cache key prefix defaults to `runfabric:daemon:api:`; override with `RUNFABRIC_DAEMON_CACHE_PREFIX` so a single Redis can serve multiple teams or projects (e.g. `RUNFABRIC_DAEMON_CACHE_PREFIX=team-a:runfabric:api:`). Per-endpoint TTL: validate 10m, resolve/plan 5m, releases 1m (or cap with `--cache-ttl`).

## OpenTelemetry

When `OTEL_EXPORTER_OTLP_ENDPOINT` or `OTEL_TRACES_ENABLED` is set, the daemon creates a span per HTTP request (method and path) and exports traces via OTLP over HTTP. See [TELEMETRY.md](TELEMETRY.md) for env vars and setup.

## Docker

Run the daemon in a container:

```bash
# Build (from repo root)
docker build -f Dockerfile.daemon -t runfabric-daemon .

# API only on port 8766
docker run -p 8766:8766 runfabric-daemon
```

**With dashboard and optional Redis** (docker-compose):

```bash
docker compose -f docker-compose.daemon.yml up -d
# API: http://localhost:8766
```

To serve the dashboard from your project, mount it and override the command: in `docker-compose.daemon.yml`, uncomment the `volumes` and `command` under the `daemon` service so the container runs with `--dashboard --config /app/runfabric.yml --workspace /app` (and optionally `--cache-url redis://redis:6379/0`). Ensure the repo root contains `runfabric.yml` when you run `docker compose -f docker-compose.daemon.yml up`.

## Endpoints

### Health and version

- **GET /healthz** — Readiness/liveness: returns `200 OK` with body `ok`. No auth. Use for Kubernetes probes or load balancer health checks.
- **GET /version** — Returns JSON `{ "version": "<engine version>", "protocol": "<protocol version>" }`. No auth.

### Config API (POST, JSON body = YAML config)

- **POST /validate** — Validate config; query `stage=<name>` optional. Response: `{ "ok": true }` or `{ "ok": false, "error": "..." }`.
- **POST /resolve** — Resolve config for stage; query `stage=<name>`. Response: resolved config JSON.
- **POST /plan** — Run plan for stage; query `stage=<name>`. Response: plan result.
- **POST /deploy** — Run deploy for stage; query `stage=<name>`. Response: deploy result.
- **POST /remove** — Run remove for stage; query `stage=<name>`. Response: remove result.
- **POST /releases** — List releases for stage; query `stage=<name>`. Response: releases list.

When `--api-key` is set, send `X-API-Key: <key>` on every request.

### Dashboard (only when `--dashboard`)

- **GET /** — HTML dashboard: project name, stage selector, last deployment info. Query `?stage=<name>` to switch stage.
- **POST /action/plan** — Run plan; query `stage=<name>`. Response: `{ "ok": true, "result": ... }` or `{ "ok": false, "error": "..." }`.
- **POST /action/deploy** — Run deploy; query `stage=<name>`.
- **POST /action/remove** — Run remove; query `stage=<name>`.

### Without dashboard

- **GET /** — JSON: `{ "service": "runfabric-daemon", "api": "POST /validate, ...", "dashboard": "run with --dashboard and --config for GET /" }`.

## Security

The daemon listens on HTTP by default. For production:

- Run behind a **reverse proxy** (nginx, Caddy, Traefik) that terminates TLS and forwards to the daemon. Use `X-Forwarded-For` / `X-Forwarded-Proto` if the app needs to know the client or scheme.
- Optionally require `--api-key` and send `X-API-Key` from clients; use `--rate-limit` to cap requests per minute per client.
- Do not expose the daemon directly to the internet without TLS. Future options may include `--tls-cert` / `--tls-key` for direct TLS.

## Running as a system service

**systemd** (Linux): set `WorkingDirectory` to your project root and pass `--config` and `--workspace .` so paths are correct.

```ini
[Unit]
Description=RunFabric daemon
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/my-runfabric-project
ExecStart=/usr/local/bin/runfabric daemon --dashboard --config runfabric.yml --workspace . --port 8766
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

**launchd** (macOS): use `WorkingDirectory` and the same flags in `ProgramArguments`.

## See also

- [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md) — Full CLI reference.
- [MCP.md](MCP.md) — MCP server for agents/IDEs (plan, deploy, doctor, invoke).
