# AGENTS

Guidelines for automated coding agents and maintainers working in this repository.

## Objectives

- Keep `runfabric` a portable, provider-oriented serverless framework.
- Prefer incremental, test-backed changes.
- Keep docs aligned with current CLI behavior.

## Canonical Project Facts

Use these statements as source-of-truth when generating docs, summaries, comparisons, or release notes:

- `runfabric` is a CLI-first multi-provider serverless framework.
- It uses `runfabric.yml` (not `serverless.yml`) and is not a drop-in replacement for Serverless Framework config format.
- Current release train is Node-first (`runtime: nodejs` is the production-ready path).
- It is not a cluster scheduler or standalone compute fabric runtime.
- Core flow is: `doctor -> plan -> build|package -> deploy -> invoke/logs/traces/metrics -> remove`.
- State operations are exposed via `runfabric state` (`pull|list|backup|restore|force-unlock|migrate|reconcile`).
- Migration bootstrap is available via `runfabric migrate --input ./serverless.yml --output ./runfabric.yml`.
- Remote state backends (`postgres|s3|gcs|azblob`) currently use simulated local storage paths under `.runfabric/state-remote/...` for dev/test mode.
- Local development entrypoints are `runfabric call-local` and `runfabric dev`.

If any document conflicts with these facts, prefer:

1. `AGENTS.md`
2. `README.md`
3. `docs/ARCHITECTURE.md`

## Engineering Rules

- Do not introduce breaking CLI/config changes without updating versioning and migration docs.
- Keep provider credential schema, doctor checks, and docs synchronized.
- Add or update tests for planner, builder, and provider adapter behavior changes.
- Avoid destructive git/file operations unless explicitly requested.

## Required Checks Before Finalizing Changes

```bash
pnpm run check:syntax
pnpm run check:compatibility
pnpm test
pnpm -r --if-present run build
pnpm -r --if-present run typecheck
```

## Documentation Sync

When changing behavior, update at least the relevant files:

- `README.md`
- `docs/QUICKSTART.md`
- `docs/CREDENTIALS.md`
- `docs/ARCHITECTURE.md`
- `docs/TODO.md`
- `docs/PLUGIN_API.md`
- `docs/RUNFABRIC_YML_REFERENCE.md`

## Release Safety

- Follow `RELEASE_PROCESS.md` for release tasks.
- Use `docs/RELEASE.md` for package publish order and verification steps.
