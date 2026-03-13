# Engine API / ABI Compatibility (P8-R2 Phase 2)

This document defines the versioned engine contract for artifact manifest v2.

## Current contract constants

- `artifact.schemaVersion`: `2`
- `engineContract.apiVersion`: `2.0.0`
- `engineContract.abiVersion`: `2.0.0`
- `engineContract.compatibilityPolicy`: `semver-minor-forward`

Source of truth is `packages/core/src/artifact-manifest.ts`.

## Artifact manifest v2 required shape

```json
{
  "schemaVersion": 2,
  "provider": "aws-lambda",
  "service": "my-service",
  "runtimeFamily": "nodejs",
  "runtimeMode": "native-compat",
  "source": {
    "entry": "src/index.ts"
  },
  "engineContract": {
    "apiVersion": "2.0.0",
    "abiVersion": "2.0.0",
    "compatibilityPolicy": "semver-minor-forward"
  },
  "build": {
    "manifestVersion": 2,
    "generatedAt": "2026-03-14T00:00:00.000Z"
  },
  "files": [
    {
      "path": "...",
      "bytes": 123,
      "sha256": "<64-char-hex>",
      "role": "entry-source|runtime-wrapper|runtime-package|manifest"
    }
  ]
}
```

## Compatibility behavior

Current validator behavior is strict for v2:

- Rejects older schema versions (downgrade):
  - `schemaVersion 1 is older than required 2`
- Rejects newer schema versions (upgrade):
  - `schemaVersion 3 is newer than supported 2`
- Rejects API/ABI or compatibility policy drift from the pinned constants.

This is intentionally fail-fast during Phase 2 to keep contract rollout deterministic.

## Phase 2 enforcement gates

- `tests/artifact-manifest-schema.test.ts`
  - validates runtime-family x runtime-mode fixture matrix
  - validates downgrade/upgrade schema rejection behavior
  - validates API/ABI/policy constants
- `npm run check:schema`
  - runs example config schema checks
  - runs artifact manifest schema compatibility tests
- `npm run check:compatibility`
  - includes `check:schema` and therefore enforces artifact manifest v2 compatibility

## Upgrade procedure

When moving from `v2` to a later manifest/contract version:

1. Update constants in `packages/core/src/artifact-manifest.ts`.
2. Update `tests/fixtures/artifact-manifest-v2/matrix.json` (or next-version fixture set).
3. Update `tests/artifact-manifest-schema.test.ts` expectations.
4. Update builder emitters if manifest fields changed.
5. Update this document and release notes.

Do not introduce silent fallback between schema versions.
