# Project TODO

Only pending work is listed here. Completed items are removed.

## P1 - Release Readiness

- Replace `release-notes/0.1.0.md.sig` placeholder (`UNSIGNED`) with a real signature using `RELEASE_NOTES_SIGNING_KEY`.
- Run first non-dry-run release workflow execution and confirm:
  - npm publish order succeeds,
  - git tag is pushed,
  - GitHub release is created from `release-notes/<version>.md`.

## P2 - Real Deploy Validation

- Add provider command fixtures/smoke checks for real deploy mode command parsing (`RUNFABRIC_*_DEPLOY_CMD`) in CI-friendly test coverage.
