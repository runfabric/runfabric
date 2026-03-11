# AGENTS.md

## Purpose

Instructions for coding agents working in the RunFabric monorepo.

## Project truths

- `runfabric` is a CLI-first multi-provider serverless framework.
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

- `apps/cli`: CLI commands and UX
- `packages/core`: shared contracts and abstractions
- `packages/planner`: parsing, validations, portability planning
- `packages/builder`: artifact assembly
- `packages/runtime-node`: Node runtime adapters
- `packages/provider-*`: provider-specific adapters
- `tests`: unit/integration tests
- `scripts`: validation/release utilities
- `docs`: product and contributor docs

## Architecture guardrails

- Keep shared packages provider-neutral.
- Put provider-specific behavior in `packages/provider-*`.
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

- `npm run release:check`

Minimum allowed lighter checks:

- docs-only: `npm run check:docs-sync`
- small code change in one package: relevant tests + `npm run check:syntax` + `npm run check:compatibility`
  Escalate to `npm run release:check` if shared contracts, schema, planner, provider behavior, or docs-sync are affected.

## Documentation triggers

- CLI/lifecycle changes -> `README.md`, `docs/QUICKSTART.md`
- credentials/doctor changes -> `docs/CREDENTIALS.md`, `docs/PROVIDER-SETUP.md`
- schema changes -> `docs/RUNFABRIC_YML_REFERENCE.md`
- architecture/plugin changes -> `docs/ARCHITECTURE.md`, `docs/PLUGIN_API.md`

## Do not

- Do not perform destructive git operations unless explicitly asked.
- Do not make unrelated formatting or cleanup changes.
- Do not change config/flag names casually.
- Do not edit release/signing artifacts unless the task is release-related.

## Final output expectations

State:

- what changed,
- what did not change,
- checks run,
- docs updated,
- remaining risks or follow-ups.
