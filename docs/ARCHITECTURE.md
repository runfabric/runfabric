# Architecture

runfabric is a CLI/serverless deployment framework with provider adapters. It is not a standalone compute scheduler/runtime fabric.

## Repo Layout

- `apps/cli`: command entrypoints and workflow orchestration.
- `packages/core`: shared contracts, state backend, provider ops helpers.
- `packages/planner`: config parsing, validation, and portability planning.
- `packages/builder`: provider-oriented artifact assembly.
- `packages/runtime-node`: runtime adapter utilities.
- `packages/provider-*`: provider adapters.
- `tests`: unit and integration tests.

## Execution Flow

1. `runfabric.yml` is loaded (with stage overrides when applicable).
2. Planner validates providers, trigger schemas (`http|cron|queue|storage|eventbridge|pubsub|kafka|rabbitmq`), runtime constraints, and capability matrix constraints.
3. Builder creates artifacts in `.runfabric/build/<provider>/<service>/`.
4. Provider deploy writes receipt in `.runfabric/deploy/<provider>/deployment.json`.
5. CLI writes provider state through the selected state backend:
  - `local` -> `.runfabric/state/<service>/<stage>/<provider>.state.json`
  - `postgres` -> real Postgres table storage (`state.postgres.schema` / `state.postgres.table`)
  - `s3|gcs|azblob` -> real object storage keys under configured prefix (`<prefix>/<service>/<stage>/<provider>.state.json`)
6. `invoke` and `logs` operate from receipt/state/log artifacts.
7. `remove` triggers provider destroy + local cleanup (+ recovery notes on failure).
8. `traces` and `metrics` use provider-native command overrides when configured, otherwise derive data from receipts + provider event logs.
9. `dev` provides local watch/build and trigger preset simulation (`http|queue|storage`).

## Core Contracts

- `ProjectConfig`: service metadata, runtime, entry, providers, triggers, functions, hooks, env, resources, workflows, secrets, extensions, state.
- `ProviderAdapter`: `validate`, `planBuild`, `build`, `planDeploy`, `deploy`, optional `provisionResources`, optional `deployWorkflows`, optional `materializeSecrets`, `invoke`, `logs`, `destroy`.
- `ProviderCredentialSchema`: required/optional env contracts.
- `StateBackend`: backend-neutral `read/write/delete/list/lock/unlock/backup/restore` abstraction.
- `DeploymentReceipt`: provider deployment metadata and endpoint output.

## Deploy Modes

- Simulated mode (default): deterministic endpoint generation + local receipt.
- Real mode (opt-in): provider command/API response parsing.

Controls:

- `RUNFABRIC_REAL_DEPLOY=1` global
- `RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1` provider-specific
- `RUNFABRIC_<PROVIDER>_DEPLOY_CMD` for command-driven real deploy
- `aws-lambda` also supports internal SDK-based real deploy without command envs (requires role ARN configuration)

## Recovery Semantics

- Deploy failures support optional rollback (`RUNFABRIC_ROLLBACK_ON_FAILURE=1`).
- Rollback uses provider `destroy` and backend state cleanup.
- Remove failures write recovery notes under `.runfabric/recovery/remove/*.json`.

## State Lifecycle

- Deploy writes transactional state lifecycle:
  - `in_progress` before provider deploy
  - `applied` on success
  - `failed` on deploy failure
- State can persist provider address maps for deployed dependencies:
  - `resourceAddresses`
  - `workflowAddresses`
  - `secretReferences` (references only; plaintext secrets are not persisted)
- Locking is token-based with TTL + stale lock recovery.
- Lock diagnostics + recovery are exposed via:
  - `runfabric state list`
  - `runfabric state force-unlock --service <name> --stage <name> --provider <name>`

## Compose

- `runfabric compose plan|deploy` resolves dependency order from compose file.
- Deploy exports shared outputs as:
  - `RUNFABRIC_OUTPUT_<SERVICE>_<PROVIDER>_ENDPOINT`

## Tooling

- Syntax: `npm run check:syntax`
- Capability sync check: `npm run check:capabilities`
- Tests: `npm test`
- Workspace type checks: `pnpm -r --if-present run typecheck`
- Compatibility checks:
  - `npm run check:schema`
  - `npm run check:provider-contracts`
- Observability commands:
  - `runfabric traces --provider <name>`
  - `runfabric metrics --provider <name>`
- Migration command:
  - `runfabric migrate --input ./serverless.yml --output ./runfabric.yml`
- Local dev loop:
  - `runfabric dev --preset http --watch`
  - `runfabric dev --preset queue --once`
  - `runfabric dev --preset storage --once`
