# Changelog

All notable changes to `runfabric` are documented in this file.

The format is based on the policy in `CHANGELOG_POLICY.md` and follows Semantic Versioning in `VERSIONING.md`.

## [0.1.0] - 2026-03-11

### Added

- Initial multi-provider framework scaffold.
- Provider adapters for AWS, GCP, Azure, Cloudflare, Vercel, Netlify, Alibaba FC, DigitalOcean Functions, Fly Machines, and IBM OpenWhisk.
- Stage-aware planning/build/deploy flow.
- Credential schema checks via `runfabric doctor`.
- Lifecycle hooks (`beforeBuild`, `afterBuild`, `beforeDeploy`, `afterDeploy`).
- Compose orchestration command with cross-service endpoint output sharing.
- Function lifecycle commands (`package`, `deploy function`, `remove`).
- Capability matrix sync automation.

### Changed

- Project identity standardized as `runfabric`.
- Local artifacts/state paths standardized under `.runfabric/`.

### Notes

- Configuration and environment variable naming are runfabric-only.
