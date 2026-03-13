# ADR 0001: Engine-First Multi-Runtime Direction

- Status: Accepted
- Date: 2026-03-14
- Scope: P8-R2 Phase 1

## Context

RunFabric supports multiple runtime families in config and provider planning, but execution behavior is still tied to provider-native language runtimes. That causes drift in behavior, packaging rules, and local/deploy parity across languages.

The target for P8-R2 is a single execution contract across languages and providers.

## Decision

RunFabric adopts an engine-first architecture:

- A compiled engine (`go` or `rust`) is the primary execution surface.
- Language support (`nodejs`, `python`, `go`, `java`, `rust`, `dotnet`) is expressed as build-time adaptation into a shared engine contract.
- Deployed artifacts should avoid language-managed runtime dependency where provider custom-runtime or container paths allow.
- Native runtime paths are retained as temporary compatibility mode (`native-compat`) during migration.

Phase 1 includes:

- strict runtime/entry fail-fast validation in parser/planner,
- a checked-in provider engine feasibility matrix,
- planner diagnostics for unsupported providers when `runtimeMode: engine`.

## Consequences

Positive:

- Deterministic behavior contract across languages.
- Earlier, clearer errors before provider deploy/runtime failures.
- Explicit provider support boundaries for engine mode.

Trade-offs:

- Additional migration work from native provider runtime flows.
- New contract versioning responsibilities for engine API/ABI and artifact schema.

## Follow-up

- Define and version engine API/ABI and artifact manifest v2 (P8-R2 Phase 2).
- Expand provider integration contracts for engine bundles (P8-R2 Phase 6).
- Replace node-only local loop with engine-backed local execution (P8-R2 Phase 7).
