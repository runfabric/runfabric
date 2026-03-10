# Release Process

This document is the maintainer runbook for shipping `runfabric` package releases.

## 1. Plan The Release

- Define target version(s) per `VERSIONING.md`.
- Confirm scope (bugfix, feature, breaking).
- Confirm docs impacted by the release.

## 2. Pre-release Validation

From repo root:

```bash
pnpm install --frozen-lockfile
pnpm run check:syntax
pnpm test
pnpm -r --if-present run build
pnpm -r --if-present run typecheck
pnpm run release:check
```

## 3. Packaging Dry Run

```bash
pnpm --filter @runfabric/cli pack --pack-destination ./.artifacts
```

If releasing multiple packages, pack each publishable package.

## 4. Publish Order

Publish in dependency order:

1. `@runfabric/core`
2. `@runfabric/planner`
3. `@runfabric/builder`
4. `@runfabric/runtime-node`
5. `@runfabric/provider-*`
6. `@runfabric/cli`

## 5. Post-release Verification

- Install released package(s) in a clean directory.
- Run Hello World from `docs/QUICKSTART.md`.
- Validate `runfabric doctor`, `runfabric plan`, `runfabric build`, `runfabric deploy`.

## 6. Communication

- Update `CHANGELOG.md` according to `CHANGELOG_POLICY.md`.
- Publish release notes with key features/fixes and migration notes.
- Link updated docs (`README.md`, `docs/MIGRATION.md`, `docs/CREDENTIALS.md`).

## 7. Rollback / Recovery

- Deprecate bad versions on npm.
- Publish a patch release with fix notes.
- Document temporary workarounds until fix is available.
