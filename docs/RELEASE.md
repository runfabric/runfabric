# Release Guide

This document is the maintainer playbook for releasing `runfabric` as npm packages.
For the root-level release runbook, see `RELEASE_PROCESS.md`.

## 1. Release Model (Finalized)

Release model is finalized as **Model B**:

- publish the full repo package set
- dependency order: `@runfabric/core` -> planner/builder/runtime/providers -> `@runfabric/cli`

## 2. Preflight Checklist

- All target package manifests are publish-ready:
  - `private` flags are correct,
  - metadata is present (`description`, `license`, `repository`, `engines`, `keywords`),
  - `publishConfig` is set as intended.
- `pnpm-lock.yaml` is committed.
- CI is green on `main`.
- `docs/QUICKSTART.md` and `docs/CREDENTIALS.md` match current CLI behavior.
- At least one provider has a real deploy path (Cloudflare API mode) and endpoint output behavior is documented.

## 3. Verification Commands

From repo root:

```bash
pnpm install --frozen-lockfile
pnpm run check:syntax
pnpm run check:capabilities
pnpm test
pnpm -r --if-present run build
pnpm -r --if-present run typecheck
```

## 4. Pack Dry Run

Validate publish artifacts before `npm publish`:

```bash
pnpm --filter @runfabric/cli pack --pack-destination ./.artifacts
```

If publishing multiple packages, run pack per publishable package.

## 5. Publish Sequence (Example)

For multi-package release, publish in dependency order:

1. `@runfabric/core`
2. `@runfabric/planner`
3. `@runfabric/builder`
4. `@runfabric/runtime-node`
5. provider packages
6. `@runfabric/cli`

For single-package release, publish only `@runfabric/cli`.

## 6. Post-Release Validation

- Install package from npm in a clean directory.
- Run Hello World flow from `docs/QUICKSTART.md`.
- Confirm `runfabric doctor`, `runfabric plan`, `runfabric build`, `runfabric deploy` work with documented credentials.
- Update `CHANGELOG.md` per `CHANGELOG_POLICY.md`.

## 7. Rollback Plan

- Deprecate bad release versions via npm deprecate notice.
- Publish fixed patch release with clear changelog notes.
- Update docs with known issues and workarounds until fixed.
