# Architecture Rules Knowledge Base

This KB explains what each architecture rule means, how it is enforced, and how to fix violations quickly.

## Source of Truth

- Human-readable rules: `misc/Rules (normalized).md`
- Enforcement code: `platform/core/policy/architecture/architecture_test.go`
- Shell gates: `make check-boundary`, `make check-architecture`, `make check-syntax`

## Rule Index

### Rule 1: `extensions/` boundary

- Intent: keep root extensions independent from internal/platform engine code.
- Enforced by:
  - `importAllowed(...)` in `architecture_test.go`
  - `make check-boundary`
- Typical violation:
  - `extensions/...` imports `internal/...` or `platform/...`
- Fix:
  - move shared contracts/helpers to `internal/<domain>/` or SDK-neutral types,
  - consume via plugin-sdk boundaries where applicable.

### Rule 2: package/platform separation

- Intent: avoid circular architecture between engine (`platform`) and public SDK/packages.
- Enforced by:
  - `importAllowed(...)` in `architecture_test.go`
- Forbidden edges:
  - `packages/... -> platform/...`
  - `packages/go/plugin-sdk/... -> platform/...`
  - `packages/go/plugin-sdk/... -> internal/...`
  - `platform/... -> packages/...`
- Fix:
  - extract shared logic into `internal/<domain>/`,
  - keep `packages` as external/public surface, not engine-coupled.

### Rule 2b: single `platform/extensions` root importer

- Intent: keep root `extensions/...` wiring centralized.
- Enforced by:
  - `TestImportGraphConstraints` in `architecture_test.go`
  - `make check-boundary`
- Allowed importer:
  - `platform/extensions/providerpolicy/providers.go`
- Fix:
  - route all root-extension wiring through providerpolicy.

### Rule 3 and Rule 4: no bridge/alias/canonical/facade regressions

- Intent: prevent wrapper layers that only forward or re-export behavior/types.
- Enforced by:
  - `TestRule4NoAliasBridgeArtifacts` in `architecture_test.go`
  - alias checks in `make check-boundary` / `make check-architecture`
- Additional hard checks:
  - bridge package paths `apps/sdkbridge` and `internal/provider/sdkbridge` must not exist.
- Example anti-patterns:
  - `type X = otherpkg.X` re-export-only files,
  - pass-through bridge/canonical/facade packages that only delegate with no owned behavior.
- Fix:
  - keep one canonical owner package,
  - call canonical package directly from consumers.

### Rule T3 (Target State): `internal/...` must not import `platform/...`

- Intent: keep `internal` as canonical core contracts/logic, not a consumer of platform-layer orchestration.
- Current status: documented target-state rule; not yet CI-enforced.
- Migration approach:
  - identify `internal -> platform` edges,
  - extract shared contracts/helpers into canonical `internal/<domain>/` packages,
  - make `platform` consume `internal`, not the reverse.

## Validation Playbook

- Fast architecture gate:
  - `go test ./platform/core/policy/architecture`
- Standard repo gate:
  - `make check-syntax`
- Full release gate:
  - `make release-check`

## Triage Checklist for New Violations

1. Identify illegal edge from test output (`file -> import`).
2. Decide canonical owner (`internal/<domain>/` unless intentionally public SDK).
3. Remove alias/facade layer; import canonical owner directly.
4. Re-run:
   - `go test ./platform/core/policy/architecture`
   - `make check-syntax`
