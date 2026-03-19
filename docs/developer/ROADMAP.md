# RunFabric Roadmap

RunFabric is a **multi-provider serverless framework package**: one config, one CLI workflow across cloud providers, deploying on managed serverless services that auto-scale and keep idle-cost overhead low. This page lists current scope and all roadmap items by phase (priority order).

---

## Quick navigation

- **What exists today**: Current: Core Framework
- **What we are actively improving**: Upcoming Work — Flat Phases
- **What comes next**: Future Track — AWS Step Functions

## 1. Current: Core Framework

What exists today:

- **Entry:** `runfabric` CLI and optional SDKs (Node, Python).
- **Config:** `runfabric.yml` — service, provider, functions, triggers, resources (managed binding), addons, layers, providerOverrides, fabric (active-active), deploy (healthCheck, scaling, strategy: all-at-once | canary | blue-green), build (order), alerts (webhook, slack, onError, onTimeout), app/org (optional grouping).
- **Providers:** AWS, GCP, Azure, Cloudflare, Vercel, Netlify, Fly, DigitalOcean, Alibaba, IBM, Kubernetes.
- **Flow:** `doctor` → `plan` → `build` / `package` → `deploy` → `invoke` / `logs` / `traces` / `metrics` → `remove`; `init` (with optional `--with-ci github-actions`); `generate function`; compose (plan/deploy/remove); fabric (deploy/status/endpoints); config-api (validate/resolve); dashboard (local UI); daemon (single process: API + optional dashboard, `--workspace` for project root). Local dev: `call-local` / `dev` with `--watch` for file-based reload. See [TROUBLESHOOTING.md](../user/TROUBLESHOOTING.md) for per-provider errors and fixes.
- **State:** Local file or optional backend (e.g. S3 + DynamoDB) for locks and deploy receipts.
- **Quality:** `make release-check` (full gate) and `make check-syntax` (fast CI); CLI tests cover doctor, plan, deploy, remove, compose, fabric, config-api, multi-cloud, scaling/health/layers, observability, dev live stream, addons, resource binding; TESTING_GUIDE covers dev mode and layers.

Deploy routing: AWS uses controlplane and deployrunner; other providers use deploy API and adapters. See [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 2. Phases (priority order)

**Phase completion:** A phase is **done** when (1) items are implemented and documented, (2) relevant tests exist and pass in CI, and (3) new flags/config are in the config or command reference.

This roadmap now lists only open work so it stays easy to read and execution-focused.

### Upcoming Work — Flat Phases (in progress)

Goal: Evolve RunFabric’s extension model into a **clean, dual-track system**:

- **Addons**: function/app-level augmentation with lifecycle hooks (Node/JS based), env injection, handler wrapping, build patching, and instrumentation.
- **Plugins**: Go-side capability implementations for providers, runtimes, and simulators, routed via a registry/resolver.

Foundations (manifests, registry, contracts, initial CLI surface) are implemented. Remaining work is grouped into flat execution phases (no sub-phases).

### Phase 17 — Interactive Prompt UX for `runfabric generate`

Goal: Make `runfabric generate` fully interactive by default (guided prompts), while preserving deterministic non-interactive CI/script usage.

Scope checklist:

- [ ] Add interactive prompt flow for `runfabric generate function` (name, language/runtime, trigger type, route/schedule/queue options, entry path).
- [ ] Add interactive prompt flow for `runfabric generate resource` and `runfabric generate addon` with sensible defaults and inline validation.
- [ ] Keep non-interactive mode first-class: explicit flags continue to bypass prompts and remain stable for automation.
- [ ] Add `--interactive` / `--no-interactive` behavior consistency across all `generate` subcommands.
- [ ] Add pre-submit validation inside prompts (naming collisions, invalid trigger config, unsupported provider/runtime combinations).
- [ ] Add preview/confirm step before file writes, with clear diff summary of generated files and config patches.
- [ ] Improve error recovery in interactive mode (re-prompt failed fields instead of exiting entire command).
- [ ] Add tests for prompt flows, non-interactive compatibility, and backward-compatible flag behavior.
- [ ] Update docs: command reference, quickstart/generate guidance, and generate proposal with interactive examples.

### Future Track — AWS Step Functions (state machines)

Goal: Add first-class support for **AWS Step Functions** as an AWS provider capability inside the extensions ecosystem, so users can deploy and operate **state machines** that orchestrate RunFabric-managed Lambdas.

**Implementation approach (extensions-based, no core pollution):**

- Config lives under **`extensions.aws-lambda.stepFunctions`** (Option B). No new top-level keys or structs in `engine/internal/config`; the AWS provider/extension reads and validates the blob. Schema can document the shape via `extensions.properties["aws-lambda"]` and a dedicated `$defs` entry.
- All Step Functions logic stays in the AWS provider/extension package (e.g. `engine/internal/extensions/provider/aws/` or external plugin). No AWS SDK imports or provider-specific branches in shared lifecycle code.
- Optional **orchestration capability interface** in provider contracts: only providers that support state machines implement it; core dispatches by capability, not by provider name.

**Config shape (recommended):**

    extensions:
      aws-lambda:
        stepFunctions:
          - name: order-workflow
            definitionPath: workflows/order.asl.json   # or inline definition
            role: arn:aws:iam::123456789012:role/StepFunctionsRole
            logging: { level: ALL }
            tracing: { enabled: true }
            bindings:                                  # optional: map ASL placeholders to function refs
              ProcessOrderArn: functions.processOrder
              NotifyArn: functions.sendNotification

**Anti-pollution rules:**

- No Step Functions structs in `config/types.go`; no `if provider == aws` in shared plan/deploy/remove.
- No AWS SDK imports outside the AWS provider module.
- Feature-gate orchestration commands by capability detection from the provider plugin.

**Phased implementation:**

- **Phase A — Config + validation only:** Parse `extensions.aws-lambda.stepFunctions`; validate name, definition/definitionPath, referenced function names exist; doctor feedback. No deploy yet.
- **Phase B — Plan:** Provider computes desired state machine resources; plan shows create/update/delete and diffs (definition hash, role, logging/tracing).
- **Phase C — Deploy/remove:** Resolve Lambda ARNs from deploy context; create/update/delete state machines; store ARN and metadata in receipts.
- **Phase D — Invoke/inspect:** Start execution, execution status/history, links/ARN in output.

**Capability interface (conceptual):** `ValidateOrchestration`, `PlanOrchestration`, `DeployOrchestration`, `RemoveOrchestration`, `StartExecution` / `DescribeExecution`. Only AWS implements initially; other providers implement the same interface for their native orchestration service.

**Provider mapping (orchestration / state machines):**

| Provider    | Native service / target                                      |
| ----------- | ----------------------------------------------------------- |
| AWS         | Step Functions                                              |
| GCP         | Workflows                                                   |
| Azure       | Durable Functions / Logic Apps (depending on target)        |
| Cloudflare  | Workflows (if/where supported)                               |

Config stays provider-scoped (e.g. `extensions.aws-lambda.stepFunctions`, `extensions.gcp-functions.workflows`, etc.); each provider defines its own extension key and definition format. Core dispatches by capability; no provider-specific branches in shared code.

**Product decisions to lock:** Definition source (inline vs file path; recommend both); function ARN binding (template vars vs explicit bindings map; recommend bindings); ownership (RunFabric manages only declared state machines); drift (reconcile on deploy with clear plan diff).

**Testing:** Unit tests for config extraction and validation; integration tests for create/update/delete and plan diff; regression tests that existing AWS deploy is unchanged when no `stepFunctions` block is present.

**Scope checklist:**

- [ ] Config + schema: declare Step Functions under `extensions.aws-lambda.stepFunctions` (name, definition or definitionPath, role, logging, tracing, optional bindings); schema `$defs` for IDE/docs; no change to `Config` struct.
- [ ] Phase A: parse and validate in AWS provider; doctor reports invalid config.
- [ ] Phase B: plan support — compute desired state machines, show create/update/delete and diffs.
- [ ] Phase C: deploy/remove — create/update/delete state machines; wire Lambda ARNs from deploy context; store metadata in receipts.
- [ ] Phase D: invoke/inspect — start execution, execution status/history, receipt metadata and console links.
- [ ] Optional orchestration capability interface in provider contracts; AWS provider implements; core dispatches by capability.

---

## 3. See also

| Doc                                                                   | Description                                                                                                      |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| [ARCHITECTURE.md](ARCHITECTURE.md)                                    | Deploy flow and provider layout.                                                                                 |
| [user/COMMAND_REFERENCE.md](../user/COMMAND_REFERENCE.md)             | CLI commands and flags.                                                                                          |
| [user/DAEMON.md](../user/DAEMON.md)                                   | Daemon: config API + optional dashboard, systemd/launchd.                                                        |
| [user/RUNFABRIC_YML_REFERENCE.md](../user/RUNFABRIC_YML_REFERENCE.md) | Config reference (resources, addons, layers, providerOverrides, deploy, build, alerts, app/org, state backends). |
| [FILE_STRUCTURE.md](FILE_STRUCTURE.md)                                | Repo file layout and package naming.                                                                             |
| [LAYOUT.md](LAYOUT.md)                                                | Repository layout (engine, packages, examples).                                                                  |
| [user/EXAMPLES_MATRIX.md](../user/EXAMPLES_MATRIX.md)                 | Provider and trigger support.                                                                                    |
| [user/DEV_LIVE_STREAM.md](../user/DEV_LIVE_STREAM.md)                 | Dev live stream (`--stream-from`, `--tunnel-url`).                                                               |
| [user/TESTING_GUIDE.md](../user/TESTING_GUIDE.md)                     | Testing with call-local, invoke, and CI.                                                                         |
| [PLUGINS.md](PLUGINS.md)                                              | Lifecycle hooks and plugin API contract.                                                                         |
| [user/ADDONS.md](../user/ADDONS.md)                                   | RunFabric Addons (config, catalog, per-function).                                                                |
| [ADDON_CONTRACT.md](ADDON_CONTRACT.md)                                | Addon implementation interface (supports, apply, AddonResult).                                                   |
| [EXTENSION_DEVELOPMENT_GUIDE.md](EXTENSION_DEVELOPMENT_GUIDE.md)      | Addon and extension development guidelines (contract, catalog, registry, testing).                               |
| [EXTERNAL_EXTENSIONS_PLAN.md](EXTERNAL_EXTENSIONS_PLAN.md)            | Plan for external plugins on disk (~/.runfabric/plugins/), install, and subprocess protocol.                     |
| [GENERATE_PROPOSAL.md](GENERATE_PROPOSAL.md)                          | P1: `runfabric generate function` (in-project scaffolding).                                                      |
| [user/CREDENTIALS.md](../user/CREDENTIALS.md)                         | Credentials and secret resolution.                                                                               |
| [user/TROUBLESHOOTING.md](../user/TROUBLESHOOTING.md)                 | Per-provider errors and fixes.                                                                                   |
| [user/TELEMETRY.md](../user/TELEMETRY.md)                             | OpenTelemetry tracing (OTLP).                                                                                    |
| [MCP.md](MCP.md)                                                      | MCP server for agents and IDEs (plan, deploy, doctor, invoke).                                                   |
