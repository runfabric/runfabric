# Addon and Extension Development Guide

This guide explains how to develop **RunFabric Addons** (function/app-level augmentation, Node/TS) and **RunFabric Extensions** (provider/runtime/simulator plugins, Go). Use it when building new addons or provider plugins.

---

## Quick navigation

- **I want env injection / handler wrapping / generated files** → Part 1 (Addon development)
- **I want to add a new provider/runtime/simulator** → Part 2 (Extension/plugin development)
- **I want lifecycle hooks in the Node CLI** → see [PLUGINS.md](PLUGINS.md) (Lifecycle hooks)

---

## Part 1: Addon development

Addons augment functions with env injection, generated files, patches, handler wrappers, and build steps. They are declared in `runfabric.yml` and, when using the Node/TS build pipeline, implement the **Addon** interface.

### 1.1 When to build an addon

- **Use addons for:** instrumentation (Sentry, Datadog), env injection, handler wrapping, generated helper files, build-time patches.
- **Do not use addons for:** provider execution, runtime packaging, deploy/remove/invoke/logs (those are **plugins**).

### 1.2 Contract (implementation interface)

Implement the **Addon** interface and **AddonResult** as defined in [ADDON_CONTRACT.md](ADDON_CONTRACT.md):

- **`name`**, **`kind: "addon"`**, **`version`**
- **`supports({ runtime, provider })`** → `boolean` (e.g. support only `nodejs` + `aws-lambda`).
- **`apply(input)`** → `Promise<AddonResult>` with optional `env`, `files`, `patches`, `handlerWrappers`, `buildSteps`, `warnings`.

Input to `apply`: `functionName`, `functionConfig`, `addonConfig`, `projectRoot`, `buildDir`, `generatedDir`.

### 1.3 Development guidelines (addons)

1. **ID and config key**  
   Use a stable addon ID (e.g. `sentry`, `datadog`) that matches the key under `addons` in `runfabric.yml`.

2. **`supports()`**  
   Return `true` only for runtimes and providers you have tested. Avoid claiming support for runtimes you don’t handle.

3. **Secrets and env**  
   Document which env vars your addon expects (e.g. `SENTRY_DSN`). Users declare them in `addons.<id>.secrets` and optionally in top-level `secrets`. Never log or expose secret values.

4. **`apply()` output**  
   - **env**: Merge with engine-injected addon secrets; avoid overwriting existing vars unless intended.  
   - **files**: Prefer writing under `generatedDir`; use stable paths so builds are reproducible.  
   - **patches**: Prefer minimal, idempotent find/replace; validate paths exist before patching.  
   - **warnings**: Use for non-fatal issues (e.g. optional feature disabled); keep messages short.

5. **Permissions**  
   In manifests we use **Permissions** (fs, env, network, cloud). Only request what the addon needs (e.g. env + network for Sentry).

6. **Testing**  
   - Unit-test `supports()` for different runtime/provider combinations.  
   - Integration-test `apply()` with a fixture `runfabric.yml` and assert on `AddonResult` (env keys, file paths, no unexpected patches).

7. **Catalog**  
   To appear in `runfabric addons list`, add an entry to the built-in catalog (`engine/internal/config/addons.go` `AddonCatalog()`) or serve a catalog JSON and set `addonCatalogUrl` in config.

### 1.4 Adding your addon to the catalog (Go engine)

- **Built-in:** Add a `AddonCatalogEntry` in `engine/internal/config/addons.go` (`AddonCatalog()`).  
- **Optional manifest:** Add a corresponding entry in `engine/internal/extensions/manifests/addon_manifest.go` (`builtinAddonManifests()`) with ID, Name, Description, Permissions.

### 1.5 References (addons)

- [ADDON_CONTRACT.md](ADDON_CONTRACT.md) — Full TypeScript interface and semantics.  
- [ADDONS.md](ADDONS.md) — Declarative usage in `runfabric.yml`.  
- [RUNFABRIC_YML_REFERENCE.md](RUNFABRIC_YML_REFERENCE.md) — `addons` and `addonCatalogUrl`.

---

## Part 2: Extension (plugin) development

Extensions are **provider**, **runtime**, or **simulator** plugins implemented in Go. They are registered in a registry and invoked by the engine for doctor, plan, deploy, remove, invoke, and logs.

### 2.1 When to build an extension (plugin)

- **Provider plugin:** You are adding a new cloud or platform (e.g. Cloudflare Workers, Vercel) that the engine should deploy to and operate.
- **Runtime plugin:** You are adding a new runtime builder/invoker (beyond the built-in nodejs/python support).
- **Simulator plugin:** You are adding a local or test double (future).

Do **not** implement provider/runtime logic as an addon; use the plugin interfaces instead.

### 2.2 Contract (provider plugin)

The recommended interface is **ProviderPlugin** (see [PLUGINS.md](PLUGINS.md)):

- **Meta()** → `ProviderMeta` (Name, Version, PluginVersion, Capabilities, SupportsRuntime, SupportsTriggers, SupportsResources).
- **ValidateConfig(ctx, req)** → error.
- **Doctor(ctx, req)** → *DoctorResult.
- **Plan(ctx, req)** → *PlanResult.
- **Deploy(ctx, req)** → *DeployResult.
- **Remove(ctx, req)** → *RemoveResult.
- **Invoke(ctx, req)** → *InvokeResult.
- **Logs(ctx, req)** → *LogsResult.

Request types (`DoctorRequest`, `PlanRequest`, etc.) carry `Config`, `Stage`, `Root`, `Function`, `Payload` as needed. Defined in `engine/internal/extensions/providers` (re-exported from `engine/internal/providers`).

### 2.3 Development guidelines (extensions)

1. **Package layout**  
   Implement under `engine/internal/extensions/provider/<name>/` (e.g. `engine/internal/extensions/provider/cloudflare/`). Include a **plugin.go** (or **provider.go**) that exports `New() providers.ProviderPlugin`.

2. **Meta()**  
   - **Name**: Must match the provider name used in config (e.g. `aws-lambda`, `gcp-functions`, `cloudflare`).  
   - **Capabilities**: List what you implement: `deploy`, `remove`, `invoke`, `logs`, `doctor`, `plan`.  
   - **SupportsRuntime** / **SupportsTriggers**: Declare what you support so tooling and docs stay accurate.

3. **ValidateConfig**  
   Use it to validate provider-specific config (region, project, account) before plan/deploy. Return a clear error if required fields are missing.

4. **Doctor**  
   Check credentials and API access (e.g. list buckets, test auth). Return human-readable checks; the CLI prints them for `runfabric plugin doctor <name>`.

5. **Plan / Deploy / Remove**  
   - Plan: Return a stable, deterministic plan (e.g. list of resources to create/update/delete).  
   - Deploy: Idempotent where possible; persist outputs (URLs, ARNs) for invoke/logs.  
   - Remove: Tear down only what your deploy created; handle missing resources gracefully.

6. **Invoke / Logs**  
   Invoke: Execute the function and return output (and optional run id). Logs: Return recent log lines for the function/stage. Use receipt metadata (e.g. deployed URLs) when needed.

7. **Errors**  
   Use the engine’s error types where applicable (e.g. `ErrProviderNotFound`). Wrap underlying API errors with context (stage, function name) so users can fix config or credentials.

8. **Testing**  
   - Unit tests for Plan (output shape, no unexpected resources).  
   - Mock or integration tests for Deploy/Remove (e.g. with a test project or local stub).  
   - Doctor tests that assert on check messages for valid/invalid credentials.

### 2.4 Registering your plugin

- **Built-in:** In `engine/internal/app/bootstrap.go`, after creating the registry, register your plugin:
  - **Legacy:** `reg.Register(mypkg.New())` if your type implements the legacy **Provider** interface.
  - **New:** `reg.RegisterPlugin(mypkg.New())` if your type implements **ProviderPlugin**.

- **Manifest (list/info/capabilities):** Add an entry in `engine/internal/extensions/manifests/provider_manifest.go` in `builtinPluginManifests()` so `runfabric plugin list` and `runfabric extension list` show your plugin.

### 2.5 Go SDK for extension development (external plugins)

When building **external** plugins (standalone binaries installed under `~/.runfabric/plugins/`), you should not import the full engine (it pulls in config, planner, and many internal packages). RunFabric will provide a **Go SDK** for extension development so you can:

- **Implement the provider contract** using SDK types only (no engine imports). The SDK defines wire-compatible request/response structs (e.g. `DoctorRequest`, `DoctorResult`) that match the engine’s protocol.
- **Run a stdio server** that the engine talks to: the SDK handles reading line-delimited JSON from stdin, dispatching to your implementation (Doctor, Plan, Deploy, Remove, Invoke, Logs), and writing JSON responses to stdout. You implement an interface; the SDK runs the loop.
- **Optionally** validate or emit `plugin.yaml` (manifest) and handle `--runfabric-protocol=stdio` / `--version` flags so your binary is a drop-in for the [external extensions layout](EXTERNAL_EXTENSIONS_PLAN.md).

**Planned SDK surface (v1):**

- **Module:** A separate Go module (e.g. `github.com/runfabric/runfabric-plugin-sdk` or in-repo `packages/go/plugin-sdk`) with minimal dependencies (stdlib, `encoding/json`).
- **Types:** Wire request/response types for Meta, ValidateConfig, Doctor, Plan, Deploy, Remove, Invoke, Logs (JSON tags aligned with engine).
- **Server:** `sdk.Serve(Provider)` or `sdk.Run()` that reads stdin line-by-line, decodes `{"method":"Plan","params":{...}}`, calls your provider, encodes `{"result":{...}}` or `{"error":{...}}`, writes to stdout.
- **Provider interface:** You implement an interface that takes the SDK’s request types and returns the SDK’s result types; no `*config.Config` or `*planner.Plan` from the engine.

**Today:** External plugins are not yet loadable; the engine only runs built-in plugins. Once [external extensions](EXTERNAL_EXTENSIONS_PLAN.md) Phase 15c (subprocess protocol) and the Go SDK are implemented, you will be able to build a provider in a separate repo, depend on the SDK, build a single binary, and install it via `runfabric extension install <id>`.

See [EXTERNAL_EXTENSIONS_PLAN.md](EXTERNAL_EXTENSIONS_PLAN.md) for the protocol (line-delimited JSON over stdio) and [ROADMAP.md](ROADMAP.md) for the “Go SDK for extension development” item.

### 2.6 References (extensions)

- [PLUGINS.md](PLUGINS.md) — Provider plugin interface, ProviderRegistry, lifecycle hooks.  
- [ARCHITECTURE.md](ARCHITECTURE.md) — Deploy flow and provider layout.  
- [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md) — `plugin list|info|doctor|capabilities`, `extension list|info|search`.  
- [EXTERNAL_EXTENSIONS_PLAN.md](EXTERNAL_EXTENSIONS_PLAN.md) — External plugins on disk, stdio protocol, install flow.

---

## Quick reference

| You want to…                    | Use                    | Contract / entrypoint                          |
|---------------------------------|------------------------|-----------------------------------------------|
| Inject env, patch files, wrap handler | **Addon** (Node/TS)   | Addon interface; `supports`, `apply`, AddonResult |
| Add a new cloud provider        | **Provider plugin** (Go) | ProviderPlugin; Meta, Doctor, Plan, Deploy, Remove, Invoke, Logs |
| Add a new runtime (build/run)   | **Runtime plugin** (Go)  | (Future) runtime interface; today use engine’s built-in runtimes |
| Run hooks before/after build/deploy | **Lifecycle hooks** (Node) | `hooks` in config; see [PLUGINS.md](PLUGINS.md) |

---

See also: [ROADMAP.md](ROADMAP.md) (Phase 15 extensions), [RUNFABRIC_YML_REFERENCE.md](RUNFABRIC_YML_REFERENCE.md) (addons and provider config).
