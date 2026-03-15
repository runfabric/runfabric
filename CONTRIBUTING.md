# Contributing Guide

Thanks for contributing to `runfabric`.

## Prerequisites

- Node.js >= 20
- Corepack enabled
- pnpm 10.x

## Local Setup

```bash
corepack enable
corepack prepare pnpm@10.5.2 --activate
pnpm install
```

## Development Workflow

1. Create a branch from `main`.
2. Make focused changes with tests.
3. Run local quality checks.
4. Open a pull request with a clear summary.

## Quality Checks

```bash
pnpm run check:syntax
pnpm test
pnpm -r --if-present run build
pnpm -r --if-present run typecheck
```

For Go code:

```bash
go build ./...
go test ./...
go test -cover ./...   # coverage report
```

**Coverage target:** Critical paths (e.g. `internal/config`, `internal/planner`, lifecycle, state) aim for ~95% coverage. CI may enforce a minimum coverage gate on these packages; see `docs/docs/ROADMAP_PHASES.md` Phase 7.

## Commit Guidance

Use clear, scoped commit messages. Suggested prefixes:

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `refactor:` non-functional code change
- `test:` tests
- `chore:` maintenance

## Pull Request Checklist

- Change is scoped and documented.
- Tests added/updated when behavior changes.
- Provider credential/config changes reflected in docs.
- `README.md` and relevant docs are updated.

## Architecture References

- `docs/REPO_DEVELOPMENT.md`
- `docs/ARCHITECTURE.md`
- `docs/CREDENTIALS.md`
- `docs/QUICKSTART.md`
- `docs/RELEASE.md`
- `RELEASE_PROCESS.md`
- `CHANGELOG_POLICY.md`
