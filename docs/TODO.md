# Project TODO

Only pending work is listed here. Completed items are removed.

## Active
- P8-R2 - Engine-First Multi-Runtime End-State

## Backlog

### P8-R2 - Engine-First Multi-Runtime End-State (Final Goal)

Goal:

- Ship a single compiled execution engine (`go` or `rust`) as the primary runtime surface.
- Treat language support (`nodejs`, `python`, `go`, `java`, `rust`, `dotnet`) as build-time adaptation to a shared engine contract.
- Prefer deploy artifacts that do not require language-managed runtimes at execution time where provider custom-runtime/container paths allow.
- Keep existing native runtime paths only as temporary compatibility mode during migration.

Scope classification:

- feature + schema/compat + builder/runtime + provider adapter + CLI UX + docs + tests/release.
- Architecture reference: `docs/ARCHITECTURE.md` section `Engine-First Target Model (P8-R2)`.
- ADR reference: `docs/adr/0001-engine-first-multi-runtime.md`.
- Feasibility matrix reference: `docs/ENGINE_FEASIBILITY_MATRIX.md`.

Non-goals for P8-R2:

- No source-to-source translation between languages (for example JS -> Python).
- No silent fallback from engine mode to native runtime mode.
- No breaking CLI/config rename without migration/versioning path.

Architecture principles:

- One execution contract: same event/context/response/error semantics for all languages.
- One artifact contract: same manifest shape regardless of language input.
- Explicit deploy mode and provider capability checks.
- Deterministic outputs, deterministic diagnostics, deterministic migration behavior.

Proposed config surface (draft, subject to ADR):

- `runtime.mode: engine | native-compat` (default target after rollout: `engine`).
- `runtime.family: nodejs | python | go | java | rust | dotnet` (source family for build adapter).
- Optional provider overrides remain provider-specific, but must not bypass global mode validation.

Engine contract targets (must be versioned):

- Input envelope:
  - request id, service/function id, trigger type, stage/provider metadata
  - normalized request/event payload
  - selected secrets/env snapshot
- Output envelope:
  - HTTP response shape (status/headers/body/base64) for HTTP-compatible triggers
  - async acknowledgment/result metadata for event triggers
- Error envelope:
  - stable code/classification (`user`, `config`, `dependency`, `internal`)
  - provider-safe message and optional diagnostics
- Observability:
  - structured logs with correlation ids
  - metrics counters/timers
  - trace hook integration points

Bundle contract targets (artifact v2):

- Required files:
  - engine binary
  - generated app descriptor (handler routing map, trigger map, env contract)
  - provider runtime metadata
  - artifact manifest v2
- Manifest fields:
  - source family + source entry metadata
  - build mode (`engine` or `native-compat`)
  - engine version + contract version
  - provider target mapping
  - generated file hashes/checksums

Phase 1 - ADR + feasibility matrix + fail-fast guardrails

- Publish ADR locking engine-first direction and deprecation policy for native-compat.
- Build provider feasibility matrix:
  - supports custom runtime binary directly
  - supports containerized execution path
  - unsupported for engine mode (explicit planner error)
- Add strict runtime-family/entry validation at parser+planner:
  - top-level runtime
  - function overrides
  - stage overrides
- Success criteria:
  - ADR accepted.
  - feasibility matrix checked in and test-covered.
  - invalid runtime/entry combos fail in `plan` with path-aware errors.

Phase 2 - Engine API/ABI + artifact schema v2

- Define versioned engine API/ABI and compatibility policy.
- Implement manifest v2 schema + validators.
- Add schema fixtures for all runtime families in both modes.
- Success criteria:
  - `check:compatibility` enforces v2 schema.
  - each adapter produces contract-valid manifests.
  - downgrade/upgrade compatibility checks documented.

Phase 3 - Build-time language adapters/codegen

- Implement language adapters that compile source conventions to engine descriptors:
  - handler detection
  - trigger binding metadata
  - env/resource declarations
- Ensure adapters are build-time only (no deployed language runtime dependency in engine mode).
- Add deterministic codegen tests and golden snapshots.
- Success criteria:
  - same logical handler contract from all supported languages.
  - reproducible generated descriptors across repeated builds.

Phase 4 - Engine core implementation

- Build engine runtime core:
  - dispatcher
  - trigger normalization
  - timeout/retry behavior mapping
  - structured error handling
- Add conformance suite shared across language adapters.
- Success criteria:
  - one fixture set passes across all runtime families.
  - response/error parity validated against contract.

Phase 5 - Builder engine bundling pipeline

- Refactor builder default path to emit engine bundles.
- Preserve `native-compat` as explicit temporary fallback.
- Add build diagnostics for missing compilers/toolchains and invalid mode/provider combos.
- Success criteria:
  - deterministic bundle output for engine mode.
  - no hidden fallback to native-compat.

Phase 6 - Provider integration for engine bundles

- Implement provider mapping for engine mode:
  - runtime value mapping or container runtime mapping per provider
  - provider-specific deploy command/env handoff
- Extend provider plan/deploy/invoke/remove contracts to consume manifest v2.
- Success criteria:
  - provider plan rejects unsupported engine-mode providers clearly.
  - provider contract tests pass for supported providers in engine mode.

Phase 7 - Local execution parity

- Replace node-only local loop with engine-backed local execution.
- Add runtime-family local fixtures for `call-local` and `dev`.
- Add parity checks between local and deployed behavior.
- Success criteria:
  - one local execution path for all supported runtime families in engine mode.
  - parity suite green.

Phase 8 - Migration and deprecation rollout

- Add migration tooling:
  - config migration to mode-aware runtime fields
  - compatibility warnings and suggested fixes
- Publish deprecation timeline for native-compat mode.
- Add release-note templates and upgrade checklist.
- Success criteria:
  - migration command produces deterministic diffs.
  - users receive clear warnings before any breaking switch.

Phase 9 - Docs/examples/release gates

- Update and keep aligned:
  - `README.md`
  - `docs/QUICKSTART.md`
  - `docs/RUNFABRIC_YML_REFERENCE.md`
  - `docs/ARCHITECTURE.md`
  - `docs/EXAMPLES_MATRIX.md`
  - `docs/site/*`
- Validation gates:
  - `npm run check:syntax`
  - `npm run check:capabilities`
  - `npm run check:docs-sync`
  - `npm run check:compatibility`
  - `npm test`
  - `npm run release:check`
- Success criteria:
  - docs describe engine mode as primary architecture.
  - all gates include engine-mode fixtures and contracts.

Recommended delivery slices (multiple commits)

1. ADR + feasibility matrix + runtime-entry fail-fast validations.
2. Engine API/ABI docs + artifact manifest v2 schema + schema tests.
3. Language adapter/codegen layer + deterministic snapshot tests.
4. Engine core + shared conformance suite.
5. Builder engine-bundle default + explicit native-compat fallback.
6. Provider engine-mode mapping + deploy/invoke contract tests.
7. Local engine-backed `call-local`/`dev` + parity tests.
8. Migration tooling + deprecation notices + docs/release updates.

### P9 - Optional IaC Resource Provisioning (Terraform / Pulumi)

- Add optional Terraform-backed provisioning mode for resources:
  - `resources.provisioner: native | terraform | pulumi`
  - `resources.terraform.dir`
  - `resources.terraform.workspace`
  - `resources.terraform.vars`
  - `resources.terraform.autoApprove` (default `false`)
- Add optional Pulumi-backed provisioning mode for resources:
  - `resources.pulumi.project`
  - `resources.pulumi.stack`
  - `resources.pulumi.workDir`
  - `resources.pulumi.config`
  - `resources.pulumi.refresh` (default `true`)
- Add IaC lifecycle CLI wrappers with provisioner-scoped commands to avoid overlap:
  - `runfabric resources plan --provisioner terraform`
  - `runfabric resources apply --provisioner terraform`
  - `runfabric resources preview --provisioner pulumi`
  - `runfabric resources up --provisioner pulumi`
  - `runfabric resources destroy --provisioner <terraform|pulumi>`
- Define state ownership boundaries to avoid dual source-of-truth:
  - Terraform state is canonical for infra resources when `resources.provisioner=terraform`
  - Pulumi state is canonical for infra resources when `resources.provisioner=pulumi`
  - runfabric state remains canonical for function deploy metadata/endpoints
  - persist Terraform/Pulumi outputs/imported references into runfabric state without copying full backend state files.
