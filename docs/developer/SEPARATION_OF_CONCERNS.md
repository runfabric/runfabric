# Separation Of Concerns

This guide defines layering rules for RunFabric engine code to keep the CLI, app orchestration, and provider implementations maintainable.

## Goals

- Keep CLI packages focused on input parsing, flag validation, and output formatting.
- Route orchestration behavior through a stable app boundary.
- Keep provider-specific behavior out of shared CLI and core orchestration paths.
- Prevent import drift that couples command handlers to low-level provider internals.

## Layering Rules

1. CLI layer (`internal/cli/*`)

- Owns Cobra command construction, flags, UX/status output, and command grouping.
- Must call orchestration through `internal/app` for deploy/plan/invoke/logs/state workflows.
- Should not directly import provider SDK packages.

2. App boundary (`internal/app`)

- Exposes `AppService` contract for CLI-facing orchestration operations.
- Delegates to workflow implementations in `platform/core/workflow/app`.
- Is the preferred seam for tests, mocks, and future alternate implementations.

3. Workflow orchestration (`platform/core/workflow/app`)

- Implements deploy lifecycle coordination and high-level operational flows.
- Selects provider routing through registries/capability resolution.
- Does not depend on Cobra or CLI presentation concerns.

4. Provider/runtime implementations (`providers/*`, `platform/extensions/*`, `internal/deploy/*`)

- Own provider-specific APIs, adapters, packaging, and capability behavior.
- Must not embed CLI command parsing logic.

## Import Guidance

- Allowed: `internal/cli/*` -> `internal/app`
- Allowed: `internal/app` -> `platform/core/workflow/app`
- Avoid: `internal/cli/*` -> provider internals or deploy engine internals when an app-boundary operation exists.

## Review Checklist

- Does the change keep command parsing in CLI and business flow in app/workflow layers?
- Are new command handlers routed through `internal/app` where applicable?
- Are docs updated when command ownership or boundaries change?
- Do tests still cover both root command wiring and command behavior paths?
