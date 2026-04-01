# Architecture Ownership ADR

Status: Accepted

## Purpose

This document freezes the canonical ownership model for shared extension-facing domains so architecture rules are explicit, reviewable, and testable.

## Decision

Shared contracts that are consumed across platform and extension boundaries must have exactly one canonical implementation area in the repository.

## Canonical ownership table

| Domain              | Canonical ownership                                                                         | Notes                                                                                                                              |
| ------------------- | ------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `provider`          | `internal/provider/contracts` and `platform/extensions/providerpolicy`                      | `providerpolicy` is the single platform-side root `extensions/...` import entrypoint.                                              |
| `router`            | `extensions/routers` for plugin implementation, `platform/workflow/app` for app DTO mapping | No internal bridge package owns router contracts.                                                                                  |
| `runtime`           | `extensions/runtimes`                                                                       | Platform consumes runtime registries through `platform/extensions/providerpolicy` and `platform/extensions/registry/resolution`.   |
| `simulator`         | `extensions/simulators`                                                                     | Platform consumes simulator registries through `platform/extensions/providerpolicy` and `platform/extensions/registry/resolution`. |
| `cli-orchestration` | `platform/workflow/app`                                                                     | `internal/cli/...` command packages must call app boundary functions, not lifecycle/recovery/source internals directly.            |

## Allowed dependency directions

- `platform/...` -> `internal/...`
- `platform/extensions/providerpolicy/providers.go` -> `extensions/...`
- `platform/extensions/providerpolicy/builtin_*.go` -> `extensions/...`
- `platform/extensions/registry/resolution/...` -> `platform/extensions/providerpolicy`
- `extensions/...` -> `packages/go/plugin-sdk/...`
- `extensions/...` -> Go standard library or external vendor SDKs

## Forbidden edges

- `extensions/...` -> `internal/...`
- `extensions/...` -> `platform/...`
- `internal/...` -> `extensions/...`
- `internal/cli/...` -> `platform/workflow/lifecycle`
- `internal/cli/...` -> `platform/workflow/recovery`
- `internal/cli/...` -> `platform/deploy/source`
- more than one non-test file outside `platform/extensions/providerpolicy/` importing root `extensions/...`
- alias-only re-export layers under `internal/extensions/...`
- bridge or delegator packages that only forward shared types or behavior between `internal` and `extensions`

## Scope lock

- Root `extensions/...` packages own built-in plugin implementations for `router`, `runtime`, and `simulator` domains.
- `platform/extensions/providerpolicy/` is the only allowed non-test importer of root `extensions/...` from inside `platform/extensions`.
- `platform/workflow/app` owns app-facing DTO translation and must not push those DTOs back into extension implementation packages.
- `internal/extensions/...` is not a canonical shared-contract layer and must not be used to reintroduce bridge packages.

## Enforcement

- `make check-boundary`
- `make check-architecture`
- `go test ./platform/core/policy/architecture/...`

## Migration notes

- If a future refactor moves canonical ownership for one of these domains, update this document, the normalized rules doc, and the architecture tests in the same change.
- Do not add compatibility alias layers as an intermediate state without a removal plan in the same PR.
