# AGENTS.md

## Purpose

Instructions for coding agents working in the RunFabric monorepo.

## Project truths

- RunFabric is a multi-provider serverless framework with a unified config and CLI workflow for services, functions, resources, and workflows.
- Uses `runfabric.yml`, not `serverless.yml`.
- Not a cluster scheduler / standalone compute fabric runtime.
- Current production-ready path is Node-first (`runtime: nodejs`).
- Core lifecycle: `doctor -> plan -> build|package -> deploy -> invoke/logs/traces/metrics -> remove`.
- State operations are under `runfabric state`.

## First actions

1. Classify task: docs-only | bugfix | feature | provider adapter | schema/compat | release.
2. Read only the owning module plus at most 2 directly related files first.
3. Define success criteria before editing.
4. Keep changes minimal and scoped.

## Repo map

- `cmd/runfabric`: CLI entrypoint
- `cmd/runfabricd`: daemon entrypoint
- `cmd/runfabricw`: worker entrypoint
- `internal/cli`: CLI commands and UX (Cobra)
- `internal/state/types`: low-level state types shared across layers
- `platform/core/contracts/provider/`: shared provider contracts (deploy/remove/invoke/logs, changeset, config)
- `platform/core/contracts/provider/codec/`: config codec helpers
- `platform/core/model/`: config, devstream, and shared domain models
- `platform/core/state/`: receipt, transaction journal, core state types
- `platform/deploy/api/`: API-based deploy/remove/invoke/logs dispatch
- `platform/deploy/controlplane/`: AWS controlplane orchestration (lock, journal, cluster ops)
- `platform/deploy/exec/`: universal deploy engine (phase/checkpoint, fault injection, journal)
- `platform/workflow/app/`: app-layer entry points for all lifecycle operations
- `platform/workflow/lifecycle/`: lifecycle operation implementations (deploy, remove, invoke, logs, plan)
- `platform/workflow/runtime/`: AI/MCP workflow execution engine (LLM clients, retry, cost tracking, typed steps)
- `platform/workflow/pipeline/`: composable deploy pipeline (deploy step, health-check, DNS sync)
- `platform/extensions/`: extension boundary — providers, routers, loaders, dispatch, providerpolicy
- `platform/daemon/`: runfabricd server, Unix socket, HTTP client
- `platform/observability/`: alerts, telemetry
- `extensions/providers/<name>/`: provider-specific adapters (built-in; each has `cmd/` for plugin binary)
- `extensions/providers/linode/`: standalone external binary plugin (own go.mod; reference for third-party authors)
- `extensions/routers/<name>/`: router-specific adapters
- `packages/`: per-runtime CLI and SDK (Node cli/sdk, Python, Go, Java, .NET); see `docs/FILE_STRUCTURE.md`
- `docs/`: documentation (user and developer guides together)

## Architecture guardrails

- Keep shared packages provider-neutral.
- Put provider-specific behavior in `providers/<name>/`.
- Do not weaken portability checks without tests and docs.
- Do not introduce breaking CLI/config changes without migration/versioning updates.

## Change matrix

### CLI behavior change

- Update tests for command behavior.
- Update `README.md` and `docs/QUICKSTART.md` if user-facing behavior changed.

### Schema/config change

- Update schema compatibility checks.
- Update `docs/RUNFABRIC_YML_REFERENCE.md`.
- Update migration/versioning docs if breaking or behaviorally significant.

### Provider adapter change

- Update provider contract checks.
- Run capabilities sync checks.
- Update provider setup/credential docs if needed.

### Docs-only

- Do not run full workspace checks unless docs describe behavior that must be re-verified.

## Required validation

Default final gate for behavior changes:

- `make release-check`

Minimum allowed lighter checks:

- docs-only: skip full build; verify doc links and code references manually or via check:docs-sync if added to Makefile.
- small code change in one package: relevant tests + `make check-syntax` (or `make lint` + `make test`).
- Escalate to `make release-check` if shared contracts, schema, planner, provider behavior, or docs-sync are affected.

## Documentation triggers

- CLI/lifecycle changes -> `README.md`, `docs/QUICKSTART.md`, `docs/COMMAND_REFERENCE.md`
- credentials/doctor changes -> `docs/CREDENTIALS.md`, `docs/PROVIDER_SETUP.md`
- schema changes -> `docs/RUNFABRIC_YML_REFERENCE.md`
- architecture/plugin changes -> `docs/ARCHITECTURE.md`, `apps/registry/docs/PLUGINS.md`

## Do not

- Do not perform destructive git operations unless explicitly asked.
- Do not make unrelated formatting or cleanup changes.
- Do not change config/flag names casually.
- Do not edit release/signing artifacts unless the task is release-related.

## Code ownership

- **Core / engine:** `platform/core/`, `platform/deploy/`, `platform/workflow/` — framework maintainers.
- **Providers:** `extensions/providers/<name>/` — per-provider owners; keep adapter logic in providers, not in `platform/`.
- **Docs:** `docs/` — keep in sync with CLI and config; see COMMAND_REFERENCE, RUNFABRIC_YML_REFERENCE, ROADMAP.
- **Docs (user):** `docs/*.md` — CLI usage + config + providers + troubleshooting.
- **Docs (developer):** `docs/` — internals + extensions/registry + repo development.
- **SDKs / packages:** `packages/node`, `packages/python`, etc. — runtime-specific owners; contract in `platform/core/contracts/provider/` and protocol docs.

## Final output expectations

State:

- what changed,
- what did not change,
- checks run,
- docs updated,
- remaining risks or follow-ups.
