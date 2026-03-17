# RunFabric Roadmap

RunFabric is a **multi-provider serverless framework package**: one config, one CLI workflow across cloud providers, deploying on managed serverless services that auto-scale and keep idle-cost overhead low. This page lists current scope and all roadmap items by phase (priority order).

---

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

### Phase 14 — AI workflows (priority: medium)

Goal: Extend RunFabric to **define, validate, deploy, run, and observe AI workflows** as a **first-class config capability inside `runfabric.yml`**, integrated into the existing lifecycle (no parallel “AI deploy path”).

Design constraints (must hold):

- **Single config:** still `runfabric.yml` (no second file).
- **Single lifecycle:** `doctor → plan → build → deploy → remove` stays primary.
- **Minimal AI utilities only:** allow `runfabric ai validate` and `runfabric ai graph` for domain introspection (not a second lifecycle).

#### 14.1 Config + schema (aiWorkflow)

- [ ] Add `aiWorkflow` to the config model and schema (enable/entrypoint/models/datasets/nodes/edges).
- [ ] Validate graph: required fields, unique node IDs, references, and edge endpoints.
- [ ] Validate supported node types via a central registry (type → schema).

#### 14.2 Node registry + DAG compiler

- [ ] Node registry for MVP types (trigger / ai / data / logic / system / human).
- [ ] DAG compiler: resolve edges, detect cycles, resolve expressions, infer execution order and sync/async execution groups.
- [ ] Deterministic compiled output (stable ordering/hashes for plan/deploy/receipts).

#### 14.3 Lifecycle integration (AI-aware core commands)

- [ ] `runfabric doctor`: validate AI providers/model refs/datasets/secrets and workflow graph when `aiWorkflow` exists.
- [ ] `runfabric plan`: compile workflow DAG and merge it into the infra/resources plan; show one combined plan.
- [ ] `runfabric build`: build workflow runtime artifacts when needed (alongside normal artifacts).
- [ ] `runfabric deploy` / `remove`: deploy/remove workflow runtime bindings and triggers using the same receipts/state model.
- [ ] `runfabric call-local` / `dev`: local workflow replay/testing and payload iteration (where possible).

#### 14.4 AI utilities (minimal, non-lifecycle)

- [ ] `runfabric ai validate [--json]`: explicit DAG/schema validation (CI-friendly).
- [ ] `runfabric ai graph [--json]`: export compiled graph for tooling/dashboard.

#### 14.5 Observability + run tracking

- [ ] Run tracker: workflow run metadata, node run metadata, retries/durations/status.
- [ ] Extend `traces`/`metrics`/`logs` to include run-level + node-level workflow visibility (with redaction where needed).
- [ ] Persist workflow hash/version and compiled metadata in receipts/state.

#### 14.6 Cost signals (follow-on)

- [ ] Track per-node model usage (tokens) and estimate per-run/per-workflow cost; surface in `metrics` and `inspect`.

---

## 3. See also

| Doc                                                      | Description                                                                                |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| [ARCHITECTURE.md](ARCHITECTURE.md)                       | Deploy flow and provider layout.                                                           |
| [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md)             | CLI commands and flags.                                                                    |
| [DAEMON.md](DAEMON.md)                                   | Daemon: config API + optional dashboard, systemd/launchd.                                   |
| [RUNFABRIC_YML_REFERENCE.md](RUNFABRIC_YML_REFERENCE.md) | Config reference (resources, addons, layers, providerOverrides, deploy, build, alerts, app/org, state backends). |
| [FILE_STRUCTURE.md](FILE_STRUCTURE.md)                   | Repo file layout and package naming.                                                       |
| [LAYOUT.md](LAYOUT.md)                                   | Repository layout (engine, packages, examples).                                            |
| [EXAMPLES_MATRIX.md](EXAMPLES_MATRIX.md)                 | Provider and trigger support.                                                              |
| [DEV_LIVE_STREAM.md](DEV_LIVE_STREAM.md)                 | Dev live stream (`--stream-from`, `--tunnel-url`).                                         |
| [TESTING_GUIDE.md](TESTING_GUIDE.md)                     | Testing with call-local, invoke, and CI.                                                   |
| [PLUGINS.md](PLUGINS.md)                                 | Lifecycle hooks and plugin API contract.                                                   |
| [GENERATE_PROPOSAL.md](GENERATE_PROPOSAL.md)             | P1: `runfabric generate function` (in-project scaffolding).                                |
| [CREDENTIALS.md](CREDENTIALS.md)                         | Credentials and secret resolution.                                                         |
| [TROUBLESHOOTING.md](TROUBLESHOOTING.md)                 | Per-provider errors and fixes.                                                             |
| [TELEMETRY.md](TELEMETRY.md)                             | OpenTelemetry tracing (OTLP).                                                              |
| [MCP.md](MCP.md)                                         | MCP server for agents and IDEs (plan, deploy, doctor, invoke).                             |
