# RunFabric Roadmap

RunFabric is a **multi-provider serverless deployment framework**. This page lists current scope and all roadmap items by phase (priority order).

---

## 1. Current: Core Framework

What exists today:

- **Entry:** `runfabric` CLI and optional SDKs (Node, Python).
- **Config:** `runfabric.yml` — service, provider, functions, triggers, resources (managed binding), addons, layers, providerOverrides, fabric (active-active), deploy (healthCheck, scaling, strategy: all-at-once | canary | blue-green), build (order), alerts (webhook, slack, onError, onTimeout), app/org (optional grouping).
- **Providers:** AWS, GCP, Azure, Cloudflare, Vercel, Netlify, Fly, DigitalOcean, Alibaba, IBM, Kubernetes.
- **Flow:** `doctor` → `plan` → `build` / `package` → `deploy` → `invoke` / `logs` / `traces` / `metrics` → `remove`; compose (plan/deploy/remove); fabric (deploy/status/endpoints); config-api (validate/resolve); dashboard (local UI); daemon (single process: API + optional dashboard, `--workspace` for project root).
- **State:** Local file or optional backend (e.g. S3 + DynamoDB) for locks and deploy receipts.
- **Quality:** `make release-check`; CLI tests cover doctor, plan, deploy, remove, compose, fabric, config-api, multi-cloud, scaling/health/layers, observability, dev live stream, addons, resource binding; TESTING_GUIDE covers dev mode and layers.

Deploy routing: AWS uses controlplane and deployrunner; other providers use deploy API and adapters. See [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 2. Phases (priority order)

Phases 1–7 are complete (core lifecycle, developer experience, observability, deploy safety, extensibility, integrations, CLI and documentation).

### Phase 8 — State (priority: high) — **Complete**

- [x] **8.1** Implement `runfabric state pull`, `list`, `backup`, `restore`, `force-unlock`, `migrate`, `reconcile` by wiring to existing backends (local, S3, DynamoDB). See [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md). Backup/restore use JSON snapshot of receipts; migrate copies receipts between backend kinds; force-unlock uses `Backends.Locks.Release`; reconcile verifies each stage loads.

### Phase 9 — Provisioning and provider depth (priority: high)

- [x] **9.1** Optional provisioning (AWS) — Implement provider-side provisioning for `resources.<key>.provision: true` (e.g. RDS or ElastiCache connection string). Config and `ResourceProvisionFn` hook exist; AWS provider calls it during deploy and injects connection strings into function env. RDS: lookup by DB instance ID, optional userEnv/passwordEnv/dbNameEnv; ElastiCache: lookup by replication group or cache cluster ID.
- [x] **9.2** GCP — Extend deploy/remove/invoke/logs beyond stubs so the GCP provider performs real deployments and matches the contract in [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md). Deploy: Cloud Functions v2 API with optional source upload (GCP_UPLOAD_BUCKET); remove/invoke use API; logs fetch from Cloud Logging (last 1h).
- [x] **9.3** Azure — Real deploy/remove/invoke via Azure Management REST API; logs: portal link + optional Log Analytics (AZURE_LOG_ANALYTICS_WORKSPACE_ID) for CLI fetch.
- [x] **9.4** Cloudflare — Real deploy/remove/invoke/logs via Cloudflare API (Workers scripts, tail).
- [x] **9.5** Vercel — Real deploy/remove/invoke/logs via Vercel API (deployments, events).
- [x] **9.6** Netlify — Real deploy/remove/invoke/logs via Netlify API (sites, deploys, log).
- [x] **9.7** Fly — Real deploy/remove/invoke/logs via Fly Machines API.
- [x] **9.8** DigitalOcean — Real deploy/remove/invoke/logs via DigitalOcean App Platform API.
- [x] **9.9** Alibaba — Real deploy/remove/invoke via Alibaba FC OpenAPI; logs: console link.
- [x] **9.10** IBM — Real deploy/remove/invoke/logs via IBM OpenWhisk REST API.
- [x] **9.11** Kubernetes — Real deploy/remove/invoke/logs via client-go (namespace, deployment, service, pod logs).

### Phase 10 — Examples and onboarding (priority: medium)

- [ ] **10.1** Add or refresh examples (e.g. under `examples/node/`), quickstart flows, and ensure [FILE_STRUCTURE.md](FILE_STRUCTURE.md) / [LAYOUT.md](LAYOUT.md) match the repo.

### Phase 11 — Release and publishing (priority: medium)

- [ ] **11.1** Version bump, changelog/release notes, `make release-check`, and publish (e.g. GoReleaser + npm) when ready to ship.

---

## 3. See also

| Doc                                                      | Description                                                                                |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| [ARCHITECTURE.md](ARCHITECTURE.md)                       | Deploy flow and provider layout.                                                           |
| [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md)             | CLI commands and flags.                                                                    |
| [DAEMON.md](DAEMON.md)                                   | Daemon: config API + optional dashboard, systemd/launchd.                                   |
| [RUNFABRIC_YML_REFERENCE.md](RUNFABRIC_YML_REFERENCE.md) | Config reference (resources, addons, layers, providerOverrides, deploy, build, alerts, app/org, state backends). |
| [EXAMPLES_MATRIX.md](EXAMPLES_MATRIX.md)                 | Provider and trigger support.                                                              |
| [DEV_LIVE_STREAM.md](DEV_LIVE_STREAM.md)                 | Dev live stream (`--stream-from`, `--tunnel-url`).                                         |
| [TESTING_GUIDE.md](TESTING_GUIDE.md)                     | Testing with call-local, invoke, and CI.                                                   |
| [PLUGIN_API.md](PLUGIN_API.md)                           | Lifecycle hooks and extensions.                                                            |
| [CREDENTIALS.md](CREDENTIALS.md)                         | Credentials and secret resolution.                                                         |
| [TELEMETRY.md](TELEMETRY.md)                             | OpenTelemetry tracing (OTLP).                                                              |
| [MCP.md](MCP.md)                                         | MCP server for agents and IDEs (plan, deploy, doctor, invoke).                             |
