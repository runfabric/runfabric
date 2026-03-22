# RunFabric Roadmap

RunFabric is reset to a fresh roadmap cycle starting at Phase 1.

## Phase 4+ (Later)

### Phase 4 - Provider Orchestration, Registry MVP, and Lifecycle Completeness

Status: planned.

Goal: expand orchestration support to GCP and Azure, ship the extension registry MVP, and close lifecycle gaps in non-AWS providers.

Scope areas:

**Track A — Provider-native orchestration (GCP + Azure)**

- GCP Cloud Workflows: deploy/remove workflow definitions, invoke executions, inspect execution status and history from `runfabric workflow` commands; wire GCP Cloud Functions ARNs into workflow definitions via bindings.
- Azure Durable Functions: deploy/remove orchestration apps, start/inspect durable function executions; wire Azure Function App resource IDs from deploy context.
- Update `OrchestrationCapable` contract and `SyncOrchestrations`/`InvokeOrchestration`/`InspectOrchestrations` for GCP and Azure provider adapters.
- Add `receiptMetadata` surfacing for GCP workflows and Azure durable functions (mirror of Phase 2 AWS pattern).

**Track B — Extension Registry MVP**

- Stand up `apps/registry/` API service: implement `/v1/extensions/resolve`, `/v1/extensions/list`, `/v1/extensions/publish` endpoints per [EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md](EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md) and [REGISTRY_API_DB_SCHEMA_MVP_V1.md](REGISTRY_API_DB_SCHEMA_MVP_V1.md).
- CLI install flow: wire `runfabric extensions extension install <id>` against live registry resolve endpoint; verify checksum and optional signature; place plugin under `RUNFABRIC_HOME/plugins`.
- CLI publish flow: end-to-end test of `publish init → upload → finalize → status` against running registry.
- Registry security hardening per [REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md](REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md): rate limiting, signed manifests, CDN artifact integrity.

**Track C — Non-AWS provider lifecycle completeness**

- GCP Functions: logs (Cloud Logging last 1h), traces (Cloud Trace summaries), and metrics (Cloud Monitoring) matching AWS parity.
- Cloudflare Workers: logs (`wrangler tail` adapter), live-stream dev auto-wire parity with AWS (auto-route tunnel URL to Worker route on `invoke dev --stream-from`).
- Azure Functions: logs (Application Insights), traces, and metrics matching AWS parity.

**Track D — Dev tooling and MCP expansion**

- GCP live-stream auto-wire: implement API Gateway-equivalent for GCP Cloud Functions (Eventarc or direct URL update) to eliminate manual tunnel wiring described in [DEV_LIVE_STREAM.md](../user/DEV_LIVE_STREAM.md).
- MCP server tool expansion: add `runfabric_generate`, `runfabric_state`, and `runfabric_workflow` tools to the MCP server per [MCP.md](MCP.md).
- `runfabric generate` provider-override interactive prompt: surface the trigger capability matrix in the interactive flow so unsupported trigger/provider combinations are caught at prompt time.

## See also

| Doc                                                                                      | Description                                                                                                      |
| ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| [ARCHITECTURE.md](ARCHITECTURE.md)                                                       | Deploy flow and provider layout.                                                                                 |
| [user/COMMAND_REFERENCE.md](../user/COMMAND_REFERENCE.md)                                | CLI commands and flags.                                                                                          |
| [user/DAEMON.md](../user/DAEMON.md)                                                      | Daemon: config API + optional dashboard, systemd/launchd.                                                        |
| [user/RUNFABRIC_YML_REFERENCE.md](../user/RUNFABRIC_YML_REFERENCE.md)                    | Config reference (resources, addons, layers, providerOverrides, deploy, build, alerts, app/org, state backends). |
| [FILE_STRUCTURE.md](FILE_STRUCTURE.md)                                                   | Repo file layout and package naming.                                                                             |
| [LAYOUT.md](LAYOUT.md)                                                                   | Repository layout (engine, packages, examples).                                                                  |
| [user/EXAMPLES_MATRIX.md](../user/EXAMPLES_MATRIX.md)                                    | Provider and trigger support.                                                                                    |
| [user/DEV_LIVE_STREAM.md](../user/DEV_LIVE_STREAM.md)                                    | Dev live stream (`--stream-from`, `--tunnel-url`).                                                               |
| [user/TESTING_GUIDE.md](../user/TESTING_GUIDE.md)                                        | Testing with call-local, invoke, and CI.                                                                         |
| [PLUGINS.md](PLUGINS.md)                                                                 | Lifecycle hooks and plugin API contract.                                                                         |
| [user/ADDONS.md](../user/ADDONS.md)                                                      | RunFabric Addons (config, catalog, per-function).                                                                |
| [ADDON_CONTRACT.md](ADDON_CONTRACT.md)                                                   | Addon implementation interface (supports, apply, AddonResult).                                                   |
| [EXTENSION_DEVELOPMENT_GUIDE.md](EXTENSION_DEVELOPMENT_GUIDE.md)                         | Addon and extension development guidelines (contract, catalog, registry, testing).                               |
| [EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md](EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md) | External plugin registry contract and install flow.                                                              |
| [GENERATE_PROPOSAL.md](GENERATE_PROPOSAL.md)                                             | Interactive `runfabric generate` UX behavior, validation, and confirmation flow.                                 |
| [user/COMMAND_REFERENCE.md](../user/COMMAND_REFERENCE.md)                                | `runfabric generate` command behavior and flags.                                                                 |
| [user/CREDENTIALS.md](../user/CREDENTIALS.md)                                            | Credentials and secret resolution.                                                                               |
| [user/TROUBLESHOOTING.md](../user/TROUBLESHOOTING.md)                                    | Per-provider errors and fixes.                                                                                   |
| [user/TELEMETRY.md](../user/TELEMETRY.md)                                                | OpenTelemetry tracing (OTLP).                                                                                    |
| [MCP.md](MCP.md)                                                                         | MCP server for agents and IDEs (plan, deploy, doctor, invoke).                                                   |
