# Contributing Guide

Thanks for contributing to `runfabric`.

## Prerequisites

- Go 1.22+

## Local Setup

Clone the repo. The Go engine and CLI live under `engine/`. Build and test via the root Makefile:

```bash
make build    # builds bin/runfabric from engine/cmd/runfabric
make test     # runs tests (from engine/)
```

## Development Workflow

1. Create a branch from `main`.
2. Make focused changes with tests.
3. Run local quality checks.
4. Open a pull request with a clear summary.

## Quality Checks

```bash
make build      # build CLI binary to bin/runfabric
make test       # run all tests
make lint       # go vet or golangci-lint
make pre-push   # format + vet + build + test + docs (same as pre-push hook)
make release-check   # build + test (default gate before merge)
go test -cover ./... # coverage report
```

## Pre-push hook (lint + validation)

To run linting and validation automatically before every `git push`, enable the Git hooks:

```bash
git config core.hooksPath .githooks
```

From then on, `git push` will run `make pre-push` (gofmt, go vet, build, tests with `-race`, and doc link checks). If any step fails, the push is aborted. To skip the hook once: `git push --no-verify`.

**Coverage target:** Critical paths (e.g. `engine/internal/config`, `engine/internal/planner`, lifecycle, state) aim for ~95% coverage. Run `make coverage` for engine coverage; run `cd packages/go/sdk && go test ./...` for Go SDK tests. Java (`packages/java/sdk`) and .NET (`packages/dotnet/sdk`) SDKs have their own test suites (`mvn test`, `dotnet test`). See the internal product roadmap for coverage targets.

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
- `docs/BUILD_AND_RELEASE.md`
- `RELEASE_PROCESS.md`
- `CHANGELOG_POLICY.md`
