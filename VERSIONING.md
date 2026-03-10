# Versioning Policy

`runfabric` follows Semantic Versioning (`MAJOR.MINOR.PATCH`) for published packages.

## Scope

- Applies to all publishable workspace packages (`@runfabric/*`).
- Applies to CLI behavior, config schema, and provider adapter contracts.

## Bump Rules

- `PATCH`: backwards-compatible bug fixes, docs-only fixes, non-breaking internal refactors.
- `MINOR`: backwards-compatible features, new provider capabilities, additive config options.
- `MAJOR`: breaking API/CLI/config changes or behavior changes requiring user migration.

## Breaking Change Criteria

A change is breaking when at least one is true:

- Existing `runfabric.yml` files stop validating without modification.
- CLI flags/outputs are removed or materially changed.
- Provider adapter interfaces require code changes in downstream integrations.
- Existing automation (CI/release scripts) must change to keep working.

## Pre-release Tags

Use pre-release tags when needed:

- `x.y.z-rc.n` for release candidates.
- `x.y.z-beta.n` for broader testing.

## Monorepo Release Strategy

- Keep related package versions synchronized for coordinated feature releases.
- Publish dependency order: core -> planner/builder/runtime/providers -> cli.
- Document noteworthy changes in release notes for each published version.

## Compatibility Notes

- Minimum supported Node.js version must be called out in `README.md` and package manifests.
- Deprecated features should remain available for at least one `MINOR` where feasible before removal in next `MAJOR`.
