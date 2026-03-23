# Changelog

All notable changes to `runfabric` are documented in this file.

The format is based on the policy in `CHANGELOG_POLICY.md` and follows Semantic Versioning in `VERSIONING.md`.

## [Unreleased]

### Added

- **runfabric generate** — In-project scaffolding: `runfabric generate function <name>` with triggers http, cron, queue. Creates handler file and patches `runfabric.yml`; supports `--trigger`, `--route`, `--schedule`, `--queue-name`, `--dry-run`, `--force`, `--no-backup`.
- **configpatch** — Safe YAML patching in `engine/internal/configpatch` (add function with backup, collision detection, dry-run plan).
- **scaffold** — Shared handler templates and function-entry builder in `engine/internal/scaffold` for init and generate.
- Unit and integration tests for generate, configpatch, and scaffold (init-then-generate flow).
- **Phase 13 — Quality and DX:** Provider contract test and doc-sync (`engine/internal/deploy/api`: `APIProviderNames()`, doc_test.go; DEPLOY_PROVIDERS.md lists API-wired providers). `make check-syntax` for fast CI (vet + build + test without -race). `runfabric call-local --serve --watch` for file watch and reload (same as dev --watch). [TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) with per-provider errors and fixes. Compose concurrency documented in COMMAND_REFERENCE. Code ownership in AGENTS.md. Real-deploy and unsafe-defaults rules in RUNFABRIC_YML_REFERENCE. `runfabric init --with-ci github-actions` adds `.github/workflows/deploy.yml`.
- **Build artifact cache** — Per-function content hash (config + handler + package.json/go.mod/requirements.txt) in `engine/internal/buildcache`; cache under `.runfabric/cache`. `runfabric build` reports cache hit/miss and stores hash; `--no-cache` forces rebuild.
- **Generate extensions** — `runfabric generate resource <name>` (resources.database|cache|queue), `generate addon <name>`, `generate provider-override <key>`; configpatch AddResource/AddAddon/AddProviderOverride and scaffold BuildResourceEntry/BuildAddonEntry/BuildProviderOverrideEntry.

### Changed

- **Examples and docs** — All example paths normalized to `examples/node/...` (e.g. `examples/node/hello-http/`, `examples/node/compose-contracts/`). [QUICKSTART.md](docs/QUICKSTART.md), [PROVIDER_SETUP.md](docs/PROVIDER_SETUP.md), and [examples/README.md](examples/README.md) updated with correct paths and quick-run commands.
- [FILE_STRUCTURE.md](docs/FILE_STRUCTURE.md) and [LAYOUT.md](docs/LAYOUT.md) updated to match current repo (engine, packages, schemas, bin, examples).

---

## [0.1.0-beta.0] - 2026-03-11

### Added

- Initial multi-provider framework scaffold.
- Provider adapters for AWS, GCP, Azure, Cloudflare, Vercel, Netlify, Alibaba FC, DigitalOcean Functions, Fly Machines, and IBM OpenWhisk.
- Stage-aware planning/build/deploy flow.
- Credential schema checks via `runfabric doctor`.
- Lifecycle hooks (`beforeBuild`, `afterBuild`, `beforeDeploy`, `afterDeploy`).
- Compose orchestration command with cross-service endpoint output sharing.
- Function lifecycle commands (`package`, `deploy function`, `remove`).
- Capability matrix sync automation.
- `runfabric init` starter templates: `api`, `worker`, `queue`, `cron`.
- Release automation workflow with ordered publish and tag/release creation.
- Signed release notes verification scripts and `release-notes/` structure.
- Docs consolidated under `docs/` (COMMAND_REFERENCE, README index).
- Compose contracts reference example under `examples/node/compose-contracts/`.

### Changed

- Project identity standardized as `runfabric`.
- Local artifacts/state paths standardized under `.runfabric/`.
- Provider adapters now expose consistent `invoke`, `logs`, and `destroy` behavior.
- Provider deploy flow supports optional command/API-backed real mode parsing.
- Deploy workflow supports optional rollback-on-failure semantics.

### Notes

- Configuration and environment variable naming are runfabric-only.
