# File Structure Optimization Analysis

## Separation of Concerns & Open/Closed Principle

---

## Executive Summary

The current repository structure has **good high-level separation** (platform/core, platform/extensions, platform/deploy) but violates **SoC and OCP** in three critical areas:

1. **CLI command layer**: 56 files in flat `internal/cli/` directory (all commands mixed)
2. **Package boundaries**: Some concerns span multiple layers without clear contracts
3. **Module responsibility creep**: Some packages handle multiple concerns

---

## Current State Assessment

### ✅ What's Well-Organized

- **Platform modules** ([platform/core](platform/core), [platform/extensions](platform/extensions), [platform/deploy](platform/deploy), [platform/runtime](platform/runtime)) — separated by responsibility
- **Extensions structure** (after recent refactor) — clear internal/providers, internal/runtimes, internal/simulators
- **Provider implementations** — each provider in isolated directory
- **Config and state** — clearly separated contracts

### ❌ Major SoC/OCP Violations

#### 1. CLI Commands in Flat Structure (CRITICAL)

**Current:**

```
internal/cli/
├── addons.go                (addon management)
├── auth.go                  (authentication)
├── backend_migrate.go       (state operations)
├── build.go                 (build phase)
├── call_local.go            (invoke operations)
├── compose.go               (orchestration)
├── config_api.go            (config management)
├── daemon.go                (daemon management)
├── dashboard.go             (UI)
├── debug.go                 (debugging)
├── deploy.go                (deploy phase)
├── dev.go                   (local development)
├── doctor.go                (validation)
├── extension.go             (plugin management)
├── extension_publish.go     (plugin publishing)
├── fabric.go                (fabric operations)
├── generate.go              (code generation)
├── init.go                  (project init)
├── inspect.go               (configuration inspection)
├── invoke.go                (function invocation)
├── list.go                  (listing operations)
├── logs.go                  (log retrieval)
├── metrics.go               (metrics operations)
├── migrate.go               (migration)
├── plan.go                  (planning phase)
├── plugin.go                (provider plugins)
├── recover.go               (recovery operations)
├── remove.go                (cleanup phase)
├── state.go                 (state operations)
├── traces.go                (trace operations)
└── ... (43 files total)
```

**Problem:**

- No semantic grouping by function
- Hard to find related commands (all deploy-related mixed)
- Violates **Single Responsibility** (one concern = one package)
- Makes testing and maintenance difficult
- No clear dependencies between command families

---

#### 2. Command Grouping Violations (SoC)

Commands should be grouped by **functional domain**, not by implementation:

| Domain                        | Commands                                              | Current Files                                                                                |
| ----------------------------- | ----------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| **Lifecycle (core workflow)** | doctor, plan, build, package, deploy, remove, recover | `doctor.go`, `plan.go`, `build.go`, `package_cmd.go`, `deploy.go`, `remove.go`, `recover.go` |
| **Invocation & Observation**  | invoke, logs, traces, metrics, call-local, dev        | `invoke.go`, `logs.go`, `traces.go`, `metrics.go`, `call_local.go`, `dev.go`                 |
| **Project Management**        | init, generate, list, inspect, compose                | `init.go`, `generate.go`, `list.go`, `inspect.go`, `compose.go`                              |
| **Configuration**             | config-api, validate                                  | `config_api.go`                                                                              |
| **Plugin/Extension System**   | extension, extension-publish, plugin, addons          | `extension.go`, `extension_publish.go`, `plugin.go`, `addons.go`                             |
| **Infrastructure State**      | state, backend-migrate, lock, unlock                  | `state.go`, `backend_migrate.go`, `lock_steal.go`, `unlock.go`                               |
| **Administrative**            | auth, daemon, dashboard, debug, docs                  | `auth.go`, `daemon.go`, `dashboard.go`, `debug.go`, `docs_cmd.go`                            |
| **Authentication**            | auth, releases                                        | `auth.go`, `releases.go`                                                                     |

**Current OCP violation:** Adding a new lifecycle stage (e.g., `validate`, `optimize`, `monitor`) requires:

1. Create new file in `internal/cli/`
2. Wire it in root command
3. Handle in `cmd/runfabric/main.go`

With grouped structure, the lifecycle package would be **open for extension** (add file to subpackage) but **closed for modification** (root command structure unchanged).

---

#### 3. Shared App Layer Not Fully Utilized (SoC)

**Architecture says:** `internal/app/` should route by provider and lifecycle.

**Reality:**

- Some CLI commands directly construct registries
- Extension resolution called in multiple places (not centralized)
- No clear "app contract" that CLI must use

**Violation:** CLI is tightly coupled to resolution logic, not going through a clean app boundary.

---

## File Structure Optimization

### Recommended: CLI Reorganization

Group commands by functional domain into subpackages:

```
internal/cli/
├── common/                   # Shared utilities (output, flags, config loading)
│   ├── flags.go             # Flag definitions
│   ├── output.go            # Table, JSON, human output
│   └── loader.go            # Config/state loading helpers
│
├── lifecycle/               # Core workflow: doctor → plan → build → deploy → remove
│   ├── doctor.go
│   ├── plan.go
│   ├── build.go
│   ├── deploy.go
│   ├── remove.go
│   ├── recover.go
│   └── recover_dry_run.go
│
├── invocation/              # Function invoke & observe: invoke, logs, traces, metrics, call-local, dev
│   ├── invoke.go
│   ├── logs.go
│   ├── traces.go
│   ├── metrics.go
│   ├── call_local.go
│   └── dev.go
│
├── project/                 # Project management: init, generate, list, inspect, compose
│   ├── init.go
│   ├── generate.go
│   ├── list.go
│   ├── inspect.go
│   └── compose.go
│
├── configuration/           # Config operations: config-api, validate
│   └── config_api.go
│
├── extensions/              # Plugin system: extension, addon, plugin management
│   ├── addons.go
│   ├── extension.go
│   ├── extension_publish.go
│   └── plugin.go
│
├── infrastructure/          # State & infrastructure: state, backend-migrate, lock/unlock
│   ├── state.go
│   ├── backend_migrate.go
│   ├── lock_steal.go
│   └── unlock.go
│
├── admin/                   # Administrative: auth, daemon, dashboard, debug, docs
│   ├── auth.go
│   ├── daemon.go
│   ├── dashboard.go
│   ├── debug.go
│   ├── docs_cmd.go
│   └── releases.go
│
├── fabric/                  # Fabric operations (cross-stage, cross-cloud)
│   └── fabric.go
│
├── root.go                  # Root command that wires all groups
├── workflow.go              # Workflow helpers (shared command patterns)
└── exitcodes.go             # Exit code definitions (shared)
```

### Separation of Concerns Achieved

**Before:**

```
import (
    "cmd/runfabric/cli/.go"      // 56 files in one namespace
)
// Hard to reason about which files are related
```

**After:**

```
import (
    cli "cmd/runfabric/internal/cli/root"          // Root command only
    lifecycle "cmd/runfabric/internal/cli/lifecycle" // Lifecycle domain
    invocation "cmd/runfabric/internal/cli/invocation" // Invocation domain
)
// Clear separation; each domain can evolve independently
```

### Open/Closed Principle Improvement

**Adding a new lifecycle stage (e.g., "validate"):**

**Before (OCP violation):**

```
// 1. Create internal/cli/validate.go
// 2. Modify internal/cli/root.go to import & register
// 3. May need changes to run order in build/deploy/remove
// Result: Closed for extension (must modify root)
```

**After (OCP compliance):**

```
// 1. Create internal/cli/lifecycle/validate.go
// 2. Register in internal/cli/lifecycle/package.go
// 3. Update internal/cli/root.go to call lifecycle.RegisterCommands()
// Result: Open for extension (lifecycle.RegisterCommands handles it)
```

---

## Platform Module Optimizations

### Issue 1: `platform/extension/` vs `platform/extensions/` Confusion

**Current:**

- `platform/extension/` (singular) — contains what?
- `platform/extensions/` (plural) — contains plugins/providers

**Problem:** Name collision creates confusion.

**Recommendation:**

```
Check if platform/extension/ is actively used:
- If empty/legacy: DELETE
- If used for extension contracts: RENAME to platform/core/contracts/extension (clearer hierarchy)
- If used for extension lifecycle: RENAME to platform/extensions/lifecycle
```

### Issue 2: `internal/app/` Should Be App Entry Point

**Current state:**

- Some CLI commands construct their own registries
- App layer exists but not fully utilized

**Recommendation:**

```
Enforce through contracts:

1. internal/app/ must provide complete app service
   - AppService with methods: Doctor(), Plan(), Build(), Deploy(), Remove(), Invoke(), GetLogs()
   - All constructor logic centralized (registry building, boundary creation, state loading)

2. CLI commands ONLY call internal/app/
   - NO direct imports from platform/extensions, platform/deploy, etc.
   - NO registry construction in CLI

3. Benefits:
   - Clear contract between CLI and engine
   - Easier to test (mock AppService)
   - Plugin system can be swapped out by replacing AppService implementation
```

### Issue 3: Layer Violations in `internal/`

**Current:**

```
internal/
├── app/          # Application routing
├── cli/          # CLI commands
├── bootstrap/    # Bootstrap logic
├── (no controlplane, deploy, deployrunner, etc.)
```

**Problem:** Control plane and deployment execution are missing from description. They're scattered.

**Better structure:**

```
internal/
├── app/              # Application layer (orchestration)
├── cli/              # CLI command implementations
├── bootstrap/        # Initialization
├── deploy/           # Deployment coordination
│   ├── api/          # Provider API dispatch
│   ├── cli/          # CLI-based deploy (legacy)
│   ├── controlplane/ # AWS-specific orchestration
│   └── exec/         # Phase engine
└── (contracts, state, etc. should be in platform/)
```

---

## Concrete Issues & Recommendations

### 1. CRITICAL: CLI Package Structure Reorganization

**Impact:** Medium (refactoring work, no logic changes)
**Priority:** HIGH
**Timeline:** 2-3 days

**Steps:**

1. Create subdirectories: lifecycle/, invocation/, project/, configuration/, extensions/, infrastructure/, admin/, fabric/, common/
2. Move files into appropriate subdirectories
3. Create package-level `register.go` in each subpackage to expose command registration
4. Update `internal/cli/root.go` to call RegisterCommands from each subpackage
5. Update all imports in `cmd/runfabric/main.go`
6. Verify all CLI tests still pass

**Files affected:** 56 files in `internal/cli/`

---

### 2. HIGH: Remove Legacy Flags & Deprecated Features

**From AGENTS.md Code Ownership:**

> - **Core / engine:** `platform/engine/cmd`, `platform/engine/internal` ...

**Problem:** References to `platform/engine/` don't exist (should be `platform/core/`).

**Check deleted items from docs updates:**

- Removed planning docs from previous cleanup (EXTERNAL_EXTENSIONS_PLAN, FRONTEND_V1_PLAN, etc.)
- Need to verify ROADMAP.md lists ONLY active items

---

### 3. MEDIUM: Clarify App Layer Contract

**Current:** `internal/app/` exists but CLI doesn't fully use it.

**Recommended changes:**

```go
// internal/app/app.go - Define clear contract
type AppService interface {
    Doctor(ctx context.Context, cfg *config.Config, stage string)
    Plan(ctx context.Context, ...)
    Deploy(ctx context.Context, ...)
    Remove(ctx context.Context, ...)
    Invoke(ctx context.Context, ...)
    GetLogs(ctx context.Context, ...)
}

// Ensure all CLI commands use this
func (cmd *DeployCmd) Run(ctx context.Context) error {
    app := internal.App() // From bootstrap/singleton
    return app.Deploy(ctx, cmd.flags...)
}
```

**Benefit:**

- Plugin system can override AppService implementation
- Testing becomes much easier
- Clear boundary between CLI and core engine

---

### 4. MEDIUM: Create `platform/contracts/` for Shared Interfaces

**Examples of scattered contracts:**

- Provider contract: `platform/core/contracts/extension/provider/`
- Runtime contract: `platform/core/contracts/runtime/`
- Simulator contract: `platform/core/contracts/simulators/`
- Addon contract: `platform/core/contracts/extension/addons/`

**Current structure lacks clarity.**

**Recommendation:**

```
platform/core/contracts/
├── extension/         # All extension-system contracts
│   ├── provider/
│   ├── runtime/
│   ├── simulator/
│   └── addon/
├── app/               # App service contract
│   ├── lifecycle.go   # Doctor, Plan, Deploy, Remove
│   ├── invocation.go  # Invoke, GetLogs, GetMetrics
│   └── config.go      # Config validation
└── platform/          # Platform-wide contracts
    ├── state.go
    ├── config.go
    └── ...
```

---

## ROADMAP.md Cleanup

### Items to Remove (Already Implemented ✅)

From analysis of codebase and recent changes:

**✅ Implemented:**

1. Extension registry system (manifests, registry, resolution)
2. Built-in provider implementations (aws, gcp, azure, cloudflare, vercel, netlify, fly, digitalocean, ibm, alibaba, kubernetes)
3. Runtime plugin system (nodejs, python registration)
4. Simulator contract and local simulator
5. Provider lifecycle hooks (doctor, plan, deploy, remove, invoke, logs)
6. Config schema and validation
7. Deployrunner with phase-based execution
8. AWS controlplane and deployexec
9. Extension registry (apps/registry/)
10. Addon system (manifest, catalog, validation)
11. Extensions restructuring (internal/providers, internal/runtimes)

**Remove from ROADMAP.md:**

- Any Phase marked as complete
- Any feature with ✅ all checklist items

---

## OCP Violations Summary & Fixes

### Problem 1: Adding Commands Requires Code Modification

```
OCP: "Open for extension, closed for modification"
Current: Adding command = modify root.go + internal/cli/ (violation)
Fix: Create subpackages that self-register commands
```

### Problem 2: Adding Provider Types Requires Core Changes

```
OCP: Provider system should accept new providers without core changes
Current: Internal providers hardcoded in registry builders (violation in some cases)
Fix: Fully API-driven provider resolution from extensions boundary
```

### Problem 3: Adding App Services Requires CLI Modification

```
OCP: New app-level features should plug in without changing CLI
Current: CLI tightly coupled to specific app implementations (violation)
Fix: Define AppService interface; CLI depends on interface not implementation
```

---

## Implementation Roadmap

### Phase 1: CLI Reorganization (HIGH PRIORITY)

- [ ] Create CLI subpackage structure
- [ ] Move files to appropriate subdirectories
- [ ] Create register.go in each subpackage
- [ ] Update root.go command wiring
- [ ] Run all CLI tests
- [ ] Update docs/COMMAND_REFERENCE.md with new organization

### Phase 2: App Layer Clarification (MEDIUM PRIORITY)

- [ ] Define AppService interface in internal/app/
- [ ] Ensure all CLI commands use AppService
- [ ] Document app contract in docs/
- [ ] Add tests for app layer isolation

### Phase 3: Platform Contracts Organization (MEDIUM PRIORITY)

- [ ] Consolidate scatter contracts into platform/core/contracts/
- [ ] Clean up deprecated contract paths
- [ ] Update imports across codebase

### Phase 4: ROADMAP.md Cleanup (LOW PRIORITY)

- [ ] Remove implemented phases
- [ ] Mark current work explicitly
- [ ] Focus on future track items

---

## Success Criteria

✅ **Achieved when:**

1. CLI commands are grouped by functional domain (lifecycle, invocation, project, admin)
2. Adding a new CLI command doesn't require modifying root.go or app layer
3. All CLI imports go through `internal/app/` interface, not direct platform imports
4. Provider system fully resolved through extension boundary
5. ROADMAP.md lists only in-progress or future work
6. All tests pass with new structure
