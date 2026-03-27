# Orchestration

RunFabric supports two workflow layers that must stay separate:

- Local RunFabric workflow runtime: executes typed steps (including AI steps) in `platform/deploy/controlplane/workflow_runtime.go`.
- Provider-native orchestration: syncs provider state machines through provider orchestration hooks.

## Control Flows

### Local workflow execution

`CLI -> app -> workflow app -> WorkflowRuntime -> TypedStepHandler -> state`

This path executes all AI steps locally through `AIStepRunner`.

### Provider orchestration sync

`CLI -> app -> provider sync hooks -> cloud orchestrator API`

This path manages cloud-native definitions and state only.

## Supported Provider Orchestrators

| Provider | Service           | Definition Shape                      | State Persistence                | Typical Integration                                                  |
| -------- | ----------------- | ------------------------------------- | -------------------------------- | -------------------------------------------------------------------- |
| AWS      | Step Functions    | ASL JSON/state machine definition     | Step Functions execution history | `SyncOrchestrations`, `InvokeOrchestration`, `InspectOrchestrations` |
| GCP      | Cloud Workflows   | YAML/JSON workflow definition         | Workflow execution API history   | `SyncOrchestrations`, `InvokeOrchestration`, `InspectOrchestrations` |
| Azure    | Durable Functions | Durable orchestrator + activity model | Durable instance state           | `SyncOrchestrations`, `InvokeOrchestration`, `InspectOrchestrations` |

## Why Compute-Only Providers Do Not Support Orchestration

Compute-only providers do not expose a first-class workflow/state-machine lifecycle matching RunFabric orchestration contracts (sync, remove, invoke, inspect). They can run functions, but they do not provide a compatible orchestration definition + durable execution API surface.

## Design Rule

Do not move AI execution into provider adapters. Keep AI execution centralized in core runtime to preserve portability and policy consistency.
