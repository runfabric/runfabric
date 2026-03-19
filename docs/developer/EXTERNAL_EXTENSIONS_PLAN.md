# Plan: External Extensions (plugins on disk)

This document plans how **external** RunFabric plugins (providers, runtimes, simulators) will be installed and loaded from the user’s home directory, complementing the current **built-in** plugins in the engine binary.

---

## Quick navigation

- **Filesystem layout**: Directory layout
- **Metadata format**: Plugin manifest (`plugin.yaml`)
- **How resolution works**: Resolution order (built-in vs external)
- **How execution works**: Loading external plugins (execution model)
- **Marketplace backend**: [EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md](EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md)
- **Commands**: CLI commands
- **What needs changing**: Engine changes + Security + Versioning + Phasing

## 1. Directory layout

User-level RunFabric data lives under **`~/.runfabric/`** (override via `RUNFABRIC_HOME`). Proposed layout:

```
~/.runfabric/
├── plugins/
│   ├── providers/
│   │   ├── aws/
│   │   │   ├── 0.1.0/
│   │   │   │   ├── plugin.yaml      # manifest (id, name, version, binary, capabilities)
│   │   │   │   ├── runfabric-provider-aws   # executable (or .exe on Windows)
│   │   │   │   └── checksums.txt    # optional: sha256 per file
│   │   │   └── 0.2.0/
│   │   │       └── ...
│   │   └── gcp/
│   │       └── 1.0.0/
│   ├── runtimes/
│   │   ├── node/
│   │   │   └── 1.0.0/
│   │   └── python/
│   │       └── 1.0.0/
│   └── simulators/
│       └── aws/
│           └── 0.1.0/
├── cache/          # build/deploy caches (e.g. layer zips, plan cache)
├── logs/            # optional: CLI/daemon logs
└── state/           # optional: global state when not using project-local .runfabric
```

**Conventions:**

- **Kind + ID + version:** `plugins/<kind>/<plugin-id>/<version>/`.  
  Example: `plugins/providers/aws/0.1.0/` → provider id `aws`, version `0.1.0`.
- **Single active version per plugin (optional):** A symlink or `current` file could point to the version to use when config does not pin one (e.g. `plugins/providers/aws/current` → `0.1.0`). Alternatively we always resolve by semver from installed versions.
- **Binary name:** Same across OS where possible (e.g. `runfabric-provider-aws`); Windows gets `runfabric-provider-aws.exe`. The manifest can name the executable explicitly.

---

## 2. Plugin manifest (`plugin.yaml`)

Each versioned directory contains a **plugin.yaml** that describes the plugin for discovery, resolution, and security.

**Proposed schema (v1):**

```yaml
# plugin.yaml
apiVersion: runfabric.io/plugin/v1
kind: provider   # provider | runtime | simulator
id: aws          # plugin ID (used in runfabric.yml provider.name / runtime)
name: AWS Lambda
description: Deploy and run functions on AWS Lambda
version: "0.1.0"
pluginVersion: 1   # contract version (engine uses this to decide how to talk to the binary)

# How the engine invokes this plugin (subprocess)
executable: runfabric-provider-aws   # relative to this directory, or path
# Optional: constraints
runtime: go        # go | node (future)
protocol: stdio    # stdio | grpc (future)

# Capabilities (for plan/list/info)
capabilities:
  runtimes: [nodejs, python]
  triggers: [http, cron, queue, storage, eventbridge]
  resources: true

# Permissions (for UX and sandboxing)
permissions:
  fs: true
  env: true
  network: true
  cloud: true

# Optional: checksums for integrity (filenames relative to this dir)
# checksums:
#   runfabric-provider-aws: sha256:...
```

- **id** must match the directory name under `plugins/<kind>/` (e.g. `aws` under `providers/`).
- **version** must match the directory name (e.g. `0.1.0`).
- **executable** is the binary the engine will run; it must be in the same directory or an absolute path.
- **checksums** (optional) can be in this file or in a separate `checksums.txt`; the loader can verify before executing.

---

## 3. Resolution order (built-in vs external)

When the engine needs a provider (or runtime/simulator) by **id**:

1. **Built-in:** If the id is provided by the engine binary (e.g. `aws-lambda`, `gcp-functions`, `vercel`), use the in-process implementation. No disk lookup.
2. **External:** Else look under `~/.runfabric/plugins/<kind>/<id>/`:
   - If **version** is specified (e.g. in config or env), use that version dir if present.
   - Else use the **latest installed version** (semver sort of dir names), or the version pointed to by `current` if we adopt that.
3. If not found: return “provider not found” (or “plugin not installed”; suggest `runfabric extension install <id>`).

So:

- **Same id in both:** Built-in wins (allows overriding with external only when we explicitly say “use external” or when built-in is disabled).
- **Only external:** Load from disk and run the plugin process.

**Current implementation notes (Phase 15b/15c):**

- **Discovery:** `runfabric extension list|info|search` merges built-in + external manifests found under `RUNFABRIC_HOME/plugins/...` (default `~/.runfabric/plugins`).
- **Precedence:** Built-in wins by default. Use `--prefer-external` or `RUNFABRIC_PREFER_EXTERNAL_PLUGINS=1` to allow external manifests to override built-ins in the merged view.
- **Pinned versions (inspection):** `runfabric extension info <id> --version <v>` selects a specific installed external version dir (best-effort).

Config could support an explicit source in the future, e.g.:

```yaml
provider:
  name: aws
  source: external   # optional: builtin | external
  version: 0.2.0    # optional: for external only
```

For v1 we can keep it simple: built-in first, then external by id (and optionally one “active” version per id).

---

## 4. Loading external plugins (execution model)

External plugins are **out-of-process** (separate binary). The engine talks to them over a **protocol** so they implement the same contract as in-process providers (Doctor, Plan, Deploy, Remove, Invoke, Logs).

**Options:**

| Approach | Pros | Cons |
|----------|------|------|
| **Subprocess + stdio JSON-RPC** | Simple, no extra deps, works everywhere | JSON only, no streaming without extra framing |
| **Subprocess + stdio gRPC** | Strongly typed, streaming | Requires protobuf and gRPC runtime |
| **HashiCorp go-plugin (gRPC)** | Mature, used by Terraform | Go-only for plugin impl, more moving parts |
| **Subprocess + line-delimited JSON** | Simple, one request/response per line, easy to stream logs | Custom small protocol |

**Recommendation for v1:** **Subprocess + line-delimited JSON over stdio.**  
- Engine spawns `~/.runfabric/plugins/providers/aws/0.1.0/runfabric-provider-aws` with args such as `--runfabric-protocol=stdio`.  
- Each request: one JSON object per line (e.g. `{"method":"Plan","params":{...}}`).  
- Each response: one JSON object per line (e.g. `{"result":{...}}` or `{"error":{...}}`).  
- Same shape as current request/result types in `engine/internal/extensions/providers/requests.go` (DoctorRequest, PlanResult, etc.).  
- Later we can add a second protocol (e.g. gRPC) and negotiate via `pluginVersion` or a handshake.

**Lifecycle:** Engine starts the process when first needed (e.g. Plan or Deploy), reuses it for the same command context, and kills it when the CLI exits or after idle timeout. No long-lived daemon required for v1.

**Current implementation note (Phase 15c):** The external provider adapter starts a **fresh subprocess per call** (Doctor/Plan/Deploy/Remove/Invoke/Logs). Process reuse + idle timeout is a follow-up optimization.

**Go SDK for plugin authors:** A separate **Go SDK** for extension development is available at `packages/go/plugin-sdk`, so plugin binaries can be built without importing engine internals. It provides wire types and a stdio server loop; you implement handlers and run the server. See [EXTENSION_DEVELOPMENT_GUIDE.md](EXTENSION_DEVELOPMENT_GUIDE.md) §2.5.

---

## 5. CLI commands

| Command | Purpose |
|---------|--------|
| `runfabric extension install <id> [--version v] [--source url]` | Install a plugin (download from marketplace or URL, extract to `plugins/<kind>/<id>/<version>/`, verify checksums). |
| `runfabric extension uninstall <id> [--version v]` | Remove an installed version (or all versions of id). |
| `runfabric extension upgrade <id>` | Upgrade to latest version (re-run install from source). |
| `runfabric extension list [--kind provider\|runtime\|simulator]` | List **all** plugins: built-in **and** external (merged). Show source (builtin vs path) and version for external. |
| `runfabric extension info <id> [--version v]` | Show manifest for id (built-in or external; if external and multiple versions, show active or specified version). |

**Install flow (v1):**

1. Resolve **source:** if `--source url` given, download from URL; else resolve id from a **marketplace index** (e.g. `https://extensions.runfabric.io/index.json` or a well-known GCS/S3 bucket). The index maps id → kind, latest version, download URL, and optional checksums.
2. **Download** archive (e.g. zip/tar.gz) to `~/.runfabric/cache/`.
3. **Extract** to `~/.runfabric/plugins/<kind>/<id>/<version>/`.
4. **Verify** checksums if present (plugin.yaml or checksums.txt).
5. **Confirm** executable exists and is runnable (optional: execute with `--version` or `--capabilities` to validate).

**Uninstall:** Delete `plugins/<kind>/<id>/<version>/` or entire `plugins/<kind>/<id>/`.

---

## 6. Engine changes (high level)

- **Discovery:** New package or functions under `engine/internal/extensions/` (e.g. `external/`) that:
  - Scan `RUNFABRIC_HOME/plugins/providers/`, `.../runtimes/`, `.../simulators/`.
  - Parse each `plugin.yaml`, validate, and return a list of external plugin manifests (id, kind, version, path, capabilities).
- **Registry merge:** When building the list for `extension list` and when resolving a provider by id:
  - **Manifests:** Merge built-in manifests (from `manifests.builtinPluginManifests()`) with external manifests from disk. For duplicates (same id), either prefer built-in or allow “external overrides built-in” via config/env.
  - **Resolution:** When `reg.Get(providerName)` is called and the name is not built-in, load the external plugin (find version, read plugin.yaml, spawn process, send requests) and wrap it in an adapter that implements the in-process `Provider` interface (same as current `apiProviderStub` pattern but talking to the subprocess).
- **Adapter:** An **external provider adapter** that holds the plugin path and version, starts the subprocess, and translates Doctor/Plan/Deploy/Remove/Invoke/Logs calls to JSON-RPC (or line-delimited JSON) and back. This adapter is registered in the same registry so lifecycle code does not need to know if a provider is built-in or external.

---

## 7. Security and integrity

- **Checksums:** Optional but recommended. Install step verifies after extract; runtime can optionally re-verify before first execution.
- **Signing (future):** Marketplace or URLs could supply a signature (e.g. cosign); CLI verifies before extract. Not required for v1.
- **Permissions:** `plugin.yaml` declares permissions (fs, env, network, cloud). Engine can use this for UX (warnings) or future sandboxing (e.g. run plugin in a restricted environment).
- **No arbitrary code in manifest:** Only an executable path; no inline scripts. Addons (Node) remain separate.

---

## 8. Versioning and compatibility

- **Plugin contract version (`pluginVersion`):** Engine can refuse to run a plugin whose `pluginVersion` is greater than what the engine supports, and warn or auto-upgrade when the engine is newer.
- **Semver for plugin version:** `version` in plugin.yaml and directory name should be semver so we can compare “latest” and do upgrade/downgrade.
- **CLI vs engine:** The binary that runs plugins is the same as the one that does `extension list`; so “engine version” is the CLI version. We can record “tested with runfabric 1.x” in the manifest or index for UX.

---

## 9. Phasing

| Phase | Scope |
|-------|--------|
| **Phase 15 (current)** | Built-in plugins only; extension list/info/search; no install. |
| **Phase 15b (this plan)** | Define layout, manifest schema, resolution order; implement discovery (read ~/.runfabric/plugins, merge with built-in for list/info). No install yet; allow “external” plugins to be **manually** placed in the directory for testing. |
| **Phase 15c** | Implement subprocess protocol (stdio, line-delimited JSON) and external provider adapter; register external providers when present on disk; test with a single external provider (e.g. a stub binary). |
| **Phase 15d** | Implement `extension install` (download from URL or marketplace index), checksums, uninstall/upgrade; document and ship. |

---

## 10. Summary

- **Layout:** `~/.runfabric/plugins/<kind>/<id>/<version>/` with `plugin.yaml`, executable, and optional checksums.
- **Manifest:** `plugin.yaml` with id, kind, version, executable, capabilities, permissions, optional checksums.
- **Resolution:** Built-in first, then external by id (and version if specified).
- **Execution:** External plugins run as subprocess; protocol = line-delimited JSON over stdio (v1).
- **CLI:** `extension install|uninstall|upgrade|list|info` with merged view of built-in + external.
- **Engine:** Discovery in `extensions/external/`, merged registry, and an external-provider adapter that talks to the subprocess.

This plan aligns with the existing Phase 15 extensions model and keeps built-in and external plugins behind the same Provider interface so deploy, plan, doctor, and invoke behave the same for both.
