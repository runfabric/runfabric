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
- `internal/cli`: CLI commands and UX (Cobra)
- `internal/app`: deploy/remove/invoke/logs/plan routing (controlplane vs deploy/api vs lifecycle)
- `internal/controlplane`: lock + journal orchestration (AWS deploy/remove)
- `internal/deploy/api`: API-based deploy/remove/invoke/logs; dispatches to `providers/<name>`
- `internal/deploy/cli`: optional CLI-based deploy (wrangler, vercel, etc.)
- `internal/deployrunner`: runs adapter BuildPlan → Plan.Execute (used by controlplane for AWS)
- `internal/deployexec`: phase engine (checkpoints, Phase list); used by AWS DeployPlan
- `internal/config`, `internal/state`, `internal/planner`, `internal/providers`: shared contracts, config, portability
- `internal/lifecycle`, `internal/backends`, `internal/transactions`: lifecycle fallback, backends, journal
- `providers/<name>`: provider-specific adapters (deploy, remove, invoke, logs; resources/; triggers/)
- `test/`: unit/integration tests
- `docs/`: documentation (split by audience: `docs/user/` and `docs/developer/`)
- `packages/`: per-runtime CLI and SDK (Node cli/sdk, Python, Go, Java, .NET); see `docs/developer/FILE_STRUCTURE.md`

## Architecture guardrails

- Keep shared packages provider-neutral.
- Put provider-specific behavior in `providers/<name>/`.
- Do not weaken portability checks without tests and docs.
- Do not introduce breaking CLI/config changes without migration/versioning updates.

## Change matrix

### CLI behavior change

- Update tests for command behavior.
- Update `README.md` and `docs/user/QUICKSTART.md` if user-facing behavior changed.

### Schema/config change

- Update schema compatibility checks.
- Update `docs/user/RUNFABRIC_YML_REFERENCE.md`.
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

- CLI/lifecycle changes -> `README.md`, `docs/user/QUICKSTART.md`, `docs/user/COMMAND_REFERENCE.md`
- credentials/doctor changes -> `docs/user/CREDENTIALS.md`, `docs/user/PROVIDER_SETUP.md`
- schema changes -> `docs/user/RUNFABRIC_YML_REFERENCE.md`
- architecture/plugin changes -> `docs/developer/ARCHITECTURE.md`, `docs/developer/PLUGINS.md`

## Do not

- Do not perform destructive git operations unless explicitly asked.
- Do not make unrelated formatting or cleanup changes.
- Do not change config/flag names casually.
- Do not edit release/signing artifacts unless the task is release-related.

## Code ownership

- **Core / engine:** `platform/engine/cmd`, `platform/engine/internal` (config, planner, state, deploy/api, controlplane, lifecycle) — framework maintainers.
- **Providers:** `platform/engine/providers/<name>` — per-provider owners; keep adapter logic in providers, not in `internal/`.
- **Docs:** `docs/` — keep in sync with CLI and config; see COMMAND_REFERENCE, RUNFABRIC_YML_REFERENCE, ROADMAP.
- **Docs (user):** `docs/user/` — CLI usage + config + providers + troubleshooting.
- **Docs (developer):** `docs/developer/` — internals + extensions/registry + repo development.
- **SDKs / packages:** `packages/node`, `packages/python`, etc. — runtime-specific owners; contract in `internal/providers` and protocol docs.

## Final output expectations

State:

- what changed,
- what did not change,
- checks run,
- docs updated,
- remaining risks or follow-ups.
