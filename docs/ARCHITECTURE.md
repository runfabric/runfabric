# Architecture

In this repo the Go engine lives at the repository root. CLI command wiring lives under `internal/cli/` (split by command domain), while app orchestration entrypoints live under `platform/workflow/app/`.

Shared contracts should live under `platform/core/contracts/` to avoid stale cross-package contract copies.

---

## Quick navigation

- **End-to-end routing**: Deploy flow (CLI -> app -> provider dispatch)
- **Visual flow**: Engine request routing diagram
- **Extension boundary**: Provider/runtime resolution boundary
- **Ownership ADR**: `docs/ARCHITECTURE_OWNERSHIP.md`
- **Binary profiles ADR**: `docs/BINARY_PROFILES_ADR.md`
- **Where provider implementations live**: Provider code layout
- **Why AWS is special**: Controlplane + phase-engine execution

## Architecture ownership

Canonical ownership and dependency direction rules for provider, router, runtime, simulator, and CLI orchestration domains are frozen in `docs/ARCHITECTURE_OWNERSHIP.md`.

Binary command ownership contracts for `runfabric`, `runfabricd`, and `runfabricw` are frozen in `docs/BINARY_PROFILES_ADR.md`.

That ADR is the source of truth for:

- canonical implementation areas,
- allowed dependency directions,
- forbidden edges,
- the single `platform/extensions` importer rule.

---

## Deploy flow: CLI -> app -> dispatch mode

### Engine request routing diagram

![RunFabric engine request routing flow](architecture_engine_flow.svg)

### 1. Entry: CLI commands

| CLI command             | File                                | Calls app boundary |
| ----------------------- | ----------------------------------- | ------------------ |
| `runfabric deploy`      | `internal/cli/lifecycle/deploy.go`  | `platform/workflow/app.Deploy` (or `DeployFromSourceURL` for `--source`) |
| `runfabric remove`      | `internal/cli/lifecycle/remove.go`  | `platform/workflow/app.Remove` |
| `runfabric invoke run`  | `internal/cli/invocation/invoke.go` | `platform/workflow/app.Invoke` |
| `runfabric invoke logs` | `internal/cli/invocation/logs.go`   | `platform/workflow/app.Logs` |

`internal/cli/common/app_service.go` provides a small app-service interface used by CLI command handlers.

### 2. App layer: bootstrap and provider dispatch

**Location:** `platform/workflow/app/`

Core pieces:

- `bootstrap.go` loads config, resolves extensions, and wires state backends.
- `provider_dispatch.go` resolves provider mode:
  - `dispatchInternal`
  - `dispatchAPI`
  - `dispatchPlugin`
- `deploy.go`, `remove.go`, `invoke.go`, and `logs.go` route behavior by mode.

### 3. Extension boundary and plugin resolution

**Location:** `platform/extensions/registry/resolution/` and `platform/extensions/providerpolicy/`

- Resolution discovers built-in + external plugins and builds provider/runtime/simulator/router boundaries.
- `platform/extensions/providerpolicy/providers.go` is the single platform importer of root `extensions/...` plugin implementations.
- Built-in precedence is preserved on ID conflicts unless explicitly overridden where supported.

### 4. Execution paths

- **API-dispatched providers** -> `platform/deploy/core/api` (`Run`, `Remove`, `Invoke`, `Logs`).
- **Internal/plugin providers** -> shared lifecycle contracts in `platform/workflow/lifecycle`.
- **AWS controlplane path** -> `platform/deploy/controlplane` orchestrates lock/journal/recovery behavior and uses `platform/deploy/exec` as the phase engine.

### 5. Recovery path

- CLI: `internal/cli/lifecycle/recover.go`
- App: `platform/workflow/app/recover.go`
- Shared recovery types/validation: `platform/workflow/recovery`

Recover operations execute through provider recovery capability with durable journal state.

---

## Workflow runtime flow: CLI -> app -> runtime -> step handler -> state

Workflow execution currently uses a single in-process durable runtime loop (not separate scheduler and dispatcher services).

### Runtime path (as implemented)

1. CLI command entrypoint (`runfabric workflow run|status|cancel|replay`) in `internal/cli/common/workflow.go`.
2. App boundary forwarding in `platform/workflow/app/workflow.go`.
3. Durable runtime loop executes in `platform/deploy/controlplane/workflow_runtime.go`.
4. Step execution dispatch (code/ai/human-approval) is handled by `platform/deploy/controlplane/workflow_typed_steps.go`.
5. Durable run/step state persists through `platform/core/state/core/runs.go` under `.runfabric/runs/<stage>/<runId>.json`.

### Scheduler vs dispatcher model (current decision)

- Keep scheduler and dispatcher as conceptual roles in docs for now.
- Implementation uses one `WorkflowRuntime` loop that performs both responsibilities:
  - selects the next executable step from durable run state,
  - executes via `WorkflowStepHandler`,
  - persists checkpoints/status transitions and retries,
  - enforces pause/cancel/replay boundaries.
- No standalone scheduler/dispatcher services are implemented in the engine at this time.

### Workflow binding layers

Two distinct binding layers exist and should not be conflated:

1. Runtime step binding (engine runtime path)
   - Binds workflow step kinds (`code`, `ai-*`, `human-approval`) to typed handler behavior in `workflow_typed_steps.go`.
   - MCP/tool/resource/prompt calls are runtime-level step concerns (policy + correlation metadata).

2. Provider orchestration binding (provider-native orchestration path)
   - Binds configured provider extensions to cloud-native orchestrators (for example AWS Step Functions, GCP Cloud Workflows, Azure Durable Functions).
   - This is provider extension/orchestration sync behavior, not the local `WorkflowRuntime` step scheduling loop.

### AI workflow execution boundary

- AI step execution is centralized in `platform/deploy/controlplane/workflow_ai_runtime.go` behind `AIStepRunner`.
- `TypedStepHandler` routes `ai-retrieval`, `ai-generate`, `ai-structured`, and `ai-eval` to `AIRunner`.
- MCP tool/resource/prompt access is executed through `workflow_mcp_runtime.go` and policy-checked in core runtime.
- Provider adapters do not execute AI steps.

### Contributor rule

Do not add AI execution logic to provider adapters. Keep AI execution in controlplane/runtime so behavior remains cloud-agnostic, testable, and policy-consistent.

---

## Provider code layout

Built-in provider implementations live under `extensions/providers/<name>/`.

### Structure

```
extensions/providers/
├── aws/
├── gcp/
├── azure/
├── cloudflare/
├── vercel/
├── netlify/
├── kubernetes/
├── alibaba/
├── digitalocean/
├── fly/
└── ibm/
```

Related built-in plugin roots:

- `extensions/runtimes/`
- `extensions/routers/`
- `extensions/simulators/`
- `extensions/secretmanagers/`
- `extensions/states/`

API dispatch wiring remains provider-neutral in `platform/deploy/core/api/` and provider policy mapping in `platform/extensions/providerpolicy/`.

### Providers wired to deploy API

- **alibaba-fc**, **azure-functions**, **cloudflare-workers**, **digitalocean-functions**, **fly-machines**, **gcp-functions**, **ibm-openwhisk**, **kubernetes**, **netlify**, **vercel**

AWS uses the controlplane path.

---

**See also:** [ARCHITECTURE_OWNERSHIP.md](ARCHITECTURE_OWNERSHIP.md), [DEPLOY_PROVIDERS.md](DEPLOY_PROVIDERS.md), [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md)
