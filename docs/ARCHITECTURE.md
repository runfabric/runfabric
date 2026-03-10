# Architecture

## Repo Layout

- `apps/cli`: command entrypoints and orchestration.
- `packages/core`: shared model and contracts.
- `packages/planner`: schema-validated config parsing and compatibility planning.
- `packages/builder`: provider-oriented artifact packaging pipeline.
- `packages/runtime-node`: runtime adapters.
- `packages/provider-*`: provider adapter packages.
- `tests`: unit/integration-style tests run with Node test runner.

## Execution Flow

1. `runfabric.yml` is loaded (with optional stage selection).
2. Planner validates provider support and trigger compatibility.
3. Planner generates:
   - per-provider plan status
   - trigger portability diagnostics
   - platform primitive compatibility diagnostics
4. Builder creates provider-specific bundles under `.runfabric/build/<provider>/<service>/`:
   - copied entry source
   - provider runtime wrapper
   - artifact manifest with file hashes
5. Provider adapter deploy writes deployment receipt to `.runfabric/deploy/<provider>/deployment.json`.
6. Deploy command writes provider state to `.runfabric/state/<service>/<stage>/<provider>.state.json`.
7. Compose deploy orchestrates multiple services in dependency order and exports endpoint outputs as env vars.

## Core Model

- `ProjectConfig`: service metadata, runtime, entry, providers, triggers, functions, hooks, resources, env, extensions.
- `ProviderAdapter`: validate, planBuild, build, planDeploy, deploy (+ optional invoke/logs/destroy).
- `ProviderCredentialSchema`: provider-specific required/optional credential env contract.
- `ProviderCapabilities`: boolean feature flags + max limits.
- `PlatformPrimitive`: higher-level primitive compatibility model.
- `StateBackend`: state abstraction (`read/write/lock/unlock`) with local file implementation.
- `LifecycleHook`: extension points for `beforeBuild`, `afterBuild`, `beforeDeploy`, `afterDeploy`.

## Current Behavior Boundaries

- Cloudflare Workers supports real API deployment (opt-in via `RUNFABRIC_CLOUDFLARE_REAL_DEPLOY=1`).
- Other providers currently return deterministic endpoint patterns and write local deployment receipts.
- Stage-aware config is supported via `stages.default` and named stage overrides.
- Provider `extensions` blocks are schema-validated for known providers.
- Deploy command returns explicit exit semantics: `0` success, `2` partial failure, `1` full failure.
- Compose orchestration is available via `runfabric compose plan|deploy`.
- Function lifecycle commands are available via `runfabric package`, `runfabric deploy function <name>`, and `runfabric remove`.
- Some commands (`invoke`, `logs`) are fully implemented only for selected providers.

## State Direction

- Local state location target: `.runfabric/state/<service>/<stage>/<provider>.state.json`.
- State payload should track resource identifiers, outputs, deployment timestamps, and schema version.
- Remote backend direction: object storage + lock primitive + metadata pointer (provider-specific implementations).
- First remote backend target: AWS S3 (versioned objects) with lock coordination and parameter-store pointer.

## Tooling and Quality

- Syntax check: `npm run check:syntax`
- Capability matrix sync check: `npm run check:capabilities`
- Tests: `npm test` (tsx-backed runner, no Node `--loader` dependency)
- CLI smoke coverage includes `doctor`, `plan`, `build`, and `deploy`.
- CI workflow: `.github/workflows/ci.yml` (validation only: frozen install + syntax + tests)
