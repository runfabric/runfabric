# RunFabric Roadmap

RunFabric is reset to a fresh roadmap cycle starting at Phase 1.

## Phase 4+ (Later)

The roadmap below is ordered by dependency, similarity, and delivery priority. Earlier phases focus on the foundation needed to make later provider and registry work safer and easier to validate.

### Quick TODO (All Phases)

- [x] Phase 4: capability model, doctor checks, and live-stream hardening completed.
- [x] Phase 5: non-AWS reversible live-route rewrite parity completed (Cloudflare native plus gateway-hook path for remaining providers).
- [x] Phase 6: implement GCP observability parity (Cloud Logging logs, Cloud Trace summaries, Cloud Monitoring metrics).
- [x] Phase 6: implement Cloudflare Workers logs adapter (`wrangler tail`).
- [x] Phase 6: implement Azure observability parity (Application Insights logs plus traces and metrics surface).
- [x] Phase 6: add provider observability tests and update user docs (`COMMAND_REFERENCE.md`, `TROUBLESHOOTING.md`, `TESTING_GUIDE.md`).
- [x] Phase 7: implement GCP Cloud Workflows deploy/remove/invoke/inspect adapter support.
- [x] Phase 7: implement Azure Durable Functions deploy/remove/invoke/inspect adapter support.
- [x] Phase 7: extend orchestration contracts (`OrchestrationCapable`, `SyncOrchestrations`, `InvokeOrchestration`, `InspectOrchestrations`) and add receipt metadata parity.
- [x] Phase 7 follow-up: add explicit Azure Durable management-plane create/delete parity (separate durable asset lifecycle from app-linked metadata sync).
- [ ] Phase 8: stand up `apps/registry` MVP endpoints (`resolve`, `list`, `publish`) and database schema.
- [ ] Phase 8: wire CLI install flow against live registry with checksum and optional signature verification.
- [ ] Phase 8: wire CLI publish flow (`init`, upload, finalize, status) against running registry.
- [ ] Phase 8: apply registry security hardening (rate limiting, signed manifests, artifact integrity checks).
- [ ] Phase 9: add MCP tools for `runfabric_generate`, `runfabric_state`, and `runfabric_workflow`.
- [ ] Phase 9: add MCP integration tests and documentation updates in `docs/developer/MCP.md`.
- [ ] Final gate: run `make release-check` after each phase completion before marking status as completed.

### Phase 4 - Dev stream capability model and live-stream hardening

Status: completed.

Goal: turn the current live-stream implementation into a clearly modeled, testable capability surface before expanding more provider-specific mutations.

- Introduced an explicit dev-stream capability model, separating `lifecycle-hook`, `conditional-mutation`, and `route-rewrite` modes so CLI output is derived from code.
- Added provider-aware live-stream doctor checks for tunnel URL validation and provider-side mutation prerequisites.
- Tightened the GCP and Cloudflare mutation flows so the CLI reports why provider-side mutation could not be applied before falling back to lifecycle-only mode.
- Added env-gated integration coverage for the AWS live-stream prepare or restore path in addition to local unit coverage.
- Added trigger-capability validation to `runfabric generate provider-override`, including interactive re-prompt behavior for unsupported provider and trigger combinations.

### Phase 5 - Non-AWS live-route rewrite parity

Status: completed.

Goal: expand from lifecycle-only support to real reversible provider routing where the control plane allows a safe mutation and rollback path.

- Implemented Cloudflare Workers live-stream route-rewrite parity with restore semantics (temporary proxy worker plus reversible route updates).
- Implemented GCP gateway-owned reversible mutation hook mode for full route rewrite when configured, while retaining conditional Cloud Functions mutation fallback.
- Implemented reversible gateway-hook route rewrite contract for Azure Functions, Vercel, Netlify, Fly Machines, Kubernetes, DigitalOcean Functions, Alibaba FC, and IBM OpenWhisk.
- Added provider capability and doctor reporting for conditional route rewrite hooks so missing prerequisites are explicit before lifecycle-only fallback.

### Phase 6 - Non-AWS observability parity

Status: completed.

Goal: close the remaining provider diagnostics gaps so the non-AWS adapters expose a more uniform lifecycle surface.

- GCP Functions: logs (Cloud Logging last 1h), traces (Cloud Trace summaries), and metrics (Cloud Monitoring) matching AWS parity.
- Cloudflare Workers: logs via `wrangler tail` adapter with API-tail fallback and explicit troubleshooting guidance.
- Azure Functions: logs (Application Insights), traces, and metrics matching AWS parity.

### Phase 7 - Provider-native orchestration for GCP and Azure

Status: completed.

Goal: extend workflow orchestration beyond AWS after provider lifecycle and observability foundations are in place.

- GCP Cloud Workflows: deploy or remove workflow definitions, invoke executions, inspect execution status and history from `runfabric workflow` commands, and wire GCP Cloud Functions ARNs into workflow definitions via bindings.
- Azure Durable Functions: deploy or remove orchestration apps, start and inspect durable function executions, and wire Azure Function App resource IDs from deploy context.
- Update `OrchestrationCapable` contract and `SyncOrchestrations`, `InvokeOrchestration`, and `InspectOrchestrations` for GCP and Azure provider adapters.
- Add `receiptMetadata` surfacing for GCP workflows and Azure durable functions, mirroring the Phase 2 AWS pattern.
- Follow-up completed: Azure durable lifecycle parity now includes explicit management-plane app-settings create/delete operations in orchestration sync/remove, instead of only app-linked metadata sync.

### Phase 8 - Extension Registry MVP

Status: planned.

Goal: ship the first end-to-end extension registry flow after core provider lifecycle work is better stabilized.

- Stand up `apps/registry/` API service with `/v1/extensions/resolve`, `/v1/extensions/list`, and `/v1/extensions/publish` endpoints per [EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md](EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md) and [REGISTRY_API_DB_SCHEMA_MVP_V1.md](REGISTRY_API_DB_SCHEMA_MVP_V1.md).
- Wire CLI install flow so `runfabric extensions extension install <id>` resolves against the live registry, verifies checksum and optional signature, and places the plugin under `RUNFABRIC_HOME/plugins`.
- Implement CLI publish flow end-to-end, including `publish init`, upload, finalize, and status against a running registry.
- Add registry security hardening per [REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md](REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md), including rate limiting, signed manifests, and CDN artifact integrity.

### Phase 9 - MCP server expansion

Status: planned.

Goal: expose more of the platform lifecycle through MCP once the underlying workflows are stable enough to automate externally.

- Add `runfabric_generate`, `runfabric_state`, and `runfabric_workflow` tools to the MCP server per [MCP.md](MCP.md).

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
