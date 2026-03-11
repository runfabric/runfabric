# Release Guide

Maintainer guide for publishing `runfabric` packages.

## 1. Prepare Version Artifacts

- Update package versions.
- Update `CHANGELOG.md` with `## [<version>]` section.
- Add `release-notes/<version>.md`.
- Sign notes:

```bash
RELEASE_NOTES_SIGNING_KEY="<key>" pnpm run release:notes:sign -- --version <version>
```

## 2. Validate Locally

```bash
pnpm install --frozen-lockfile
pnpm run check:compatibility
pnpm run release:check
pnpm run release:notes:verify -- --version <version>
```

## 3. Run Release Workflow

Use GitHub Actions workflow `release`:

- input `version`
- input `publish` (true/false)
- input `dry_run` (true/false)

Workflow gates:

- `release:check`
- signed release notes verification

If `publish=true` and `dry_run=false` it will:

1. publish packages in dependency order
2. create git tag `v<version>`
3. create GitHub release using `release-notes/<version>.md`
4. verify npm publish + git tag + GitHub release body via `release:verify:published`

Manual equivalent:

```bash
pnpm run release:verify:published -- --version <version>
```

## 4. Publish Order

The automation uses this order:

1. `@runfabric/core`
2. `@runfabric/planner`
3. `@runfabric/builder`
4. `@runfabric/runtime-node`
5. all `@runfabric/provider-*`
6. `@runfabric/cli`

## 5. Required Secrets

- `NPM_TOKEN`
- `RELEASE_NOTES_SIGNING_KEY`
- `GITHUB_TOKEN` (provided automatically in GitHub Actions, required for API-based release body verification)
