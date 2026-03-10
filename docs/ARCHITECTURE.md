# Architecture

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
2. Planner validates providers, triggers, and capability constraints.
3. Builder creates artifacts in `.runfabric/build/<provider>/<service>/`.
4. Provider deploy writes receipt in `.runfabric/deploy/<provider>/deployment.json`.
5. CLI writes provider state in `.runfabric/state/<service>/<stage>/<provider>.state.json`.
6. `invoke` and `logs` operate from receipt/state/log artifacts.
7. `remove` triggers provider destroy + local cleanup (+ recovery notes on failure).

## Core Contracts

- `ProjectConfig`: service metadata, runtime, entry, providers, triggers, functions, hooks, env, extensions.
- `ProviderAdapter`: `validate`, `planBuild`, `build`, `planDeploy`, `deploy`, `invoke`, `logs`, `destroy`.
- `ProviderCredentialSchema`: required/optional env contracts.
- `StateBackend`: local state read/write/lock/unlock abstraction.
- `DeploymentReceipt`: provider deployment metadata and endpoint output.

## Deploy Modes

- Simulated mode (default): deterministic endpoint generation + local receipt.
- Real mode (opt-in): provider command/API response parsing.

Controls:

- `RUNFABRIC_REAL_DEPLOY=1` global
- `RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1` provider-specific
- `RUNFABRIC_<PROVIDER>_DEPLOY_CMD` for command-driven real deploy

## Recovery Semantics

- Deploy failures support optional rollback (`RUNFABRIC_ROLLBACK_ON_FAILURE=1`).
- Rollback uses provider `destroy` and cleans receipt/state artifacts.
- Remove failures write recovery notes under `.runfabric/recovery/remove/*.json`.

## Compose

- `runfabric compose plan|deploy` resolves dependency order from compose file.
- Deploy exports shared outputs as:
  - `RUNFABRIC_OUTPUT_<SERVICE>_<PROVIDER>_ENDPOINT`

## Tooling

- Syntax: `npm run check:syntax`
- Capability sync check: `npm run check:capabilities`
- Tests: `npm test`
- Workspace type checks: `pnpm -r --if-present run typecheck`
