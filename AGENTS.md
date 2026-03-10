# AGENTS

Guidelines for automated coding agents and maintainers working in this repository.

## Objectives

- Keep `runfabric` a portable, provider-oriented serverless framework.
- Prefer incremental, test-backed changes.
- Keep docs aligned with current CLI behavior.

## Engineering Rules

- Do not introduce breaking CLI/config changes without updating versioning and migration docs.
- Keep provider credential schema, doctor checks, and docs synchronized.
- Add or update tests for planner, builder, and provider adapter behavior changes.
- Avoid destructive git/file operations unless explicitly requested.

## Required Checks Before Finalizing Changes

```bash
pnpm run check:syntax
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

## Release Safety

- Follow `RELEASE_PROCESS.md` for release tasks.
- Use `docs/RELEASE.md` for package publish order and verification steps.
