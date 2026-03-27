## RunFabric Plugins

Plugins are **system/capability-level implementations** (providers, runtimes, simulators) that the engine can route through a registry/resolver. In v1, RunFabric ships all provider/runtime implementations bundled, but they should be treated as **plugins by design** so they can be externalized later.

- **Use plugins for**: providers, runtimes, simulators.
- **Do not use plugins for**: Sentry/Datadog wrapping, simple build hooks, or project notifications (those belong in addons).

There is also an existing **Node hook mechanism** (top-level `hooks` in `runfabric.yml`). It is treated as the **addon lifecycle hook surface**; provider/runtime/simulator plugins are Go-side extension contracts.

**Go CLI support:** The Go binary treats providers/runtimes/simulators as in-process implementations resolved through the extension boundary in `engine/internal/extensions/resolution` (provider registry + runtime/simulator resolution). It does **not** execute Node hook modules.

**Node CLI support:** The Node CLI (`@runfabric/cli`) executes lifecycle hook modules referenced via `hooks:` in config. Think of those as **addon hooks**, not provider plugins.

---

## Quick navigation

- **Go provider plugins**: Provider plugin interface
- **Engine boundary**: `internal/extensions/resolution`
- **Node lifecycle hooks**: Lifecycle hooks + Hook API contract

## Engine extension boundary (Go)

Provider/runtime resolution in the Go engine should happen through the app bootstrap + extension registry path (`internal/app/bootstrap.go`, `platform/extensions/registry/loader`):

- Build a provider registry with built-ins + API-backed provider adapters.
- Merge discoverable external plugins from `RUNFABRIC_HOME/plugins` while preserving built-in precedence.
- Keep `aws` and `aws-lambda` internal until the provider plugin contract is stable.

## Provider plugin interface (Go, recommended)

The recommended interface for internal provider plugins is **ProviderPlugin** (context + request/result). It is defined in `internal/provider/contracts` and re-exported from `platform/extensions/registry/loader/providers`.

**Interface:**

- **Meta()** → `ProviderMeta` (Name, Version, PluginVersion, Capabilities, SupportsRuntime, SupportsTriggers, SupportsResources)
- **ValidateConfig(ctx, req)** → error
- **Doctor(ctx, req)** → \*DoctorResult
- **Plan(ctx, req)** → \*PlanResult
- **Deploy(ctx, req)** → \*DeployResult
- **Remove(ctx, req)** → \*RemoveResult
- **Invoke(ctx, req)** → \*InvokeResult
- **Logs(ctx, req)** → \*LogsResult

Request types: `ValidateConfigRequest`, `DoctorRequest`, `PlanRequest`, `DeployRequest`, `RemoveRequest`, `InvokeRequest`, `LogsRequest` (each carries Config, Stage, Root, etc. as needed).

**Registry:** `ProviderRegistry` has `Register(ProviderPlugin)`, `Get(name) (ProviderPlugin, bool)`, `List() []ProviderMeta`.

**Usage:** Implement `ProviderPlugin` and register at boot:

```go
// In engine/providers/cloudflare/plugin.go
package cloudflare

func New() providers.ProviderPlugin {
    return &Plugin{}
}

type Plugin struct{}

func (p *Plugin) Meta() providers.ProviderMeta { ... }
func (p *Plugin) ValidateConfig(...) error { ... }
func (p *Plugin) Doctor(...) (*providers.DoctorResult, error) { ... }
func (p *Plugin) Plan(...) (*providers.PlanResult, error) { ... }
func (p *Plugin) Deploy(...) (*providers.DeployResult, error) { ... }
func (p *Plugin) Remove(...) (*providers.RemoveResult, error) { ... }
func (p *Plugin) Invoke(...) (*providers.InvokeResult, error) { ... }
func (p *Plugin) Logs(...) (*providers.LogsResult, error) { ... }

// In app bootstrap (or equivalent):
registry.RegisterPlugin(cloudflare.New())
registry.RegisterPlugin(vercel.New())
```

Built-in providers use the `ProviderPlugin` interface directly.

**Development:** For extension (provider plugin) development guidelines (layout, Meta, Doctor, Plan/Deploy/Remove, registration, testing), see [EXTENSION_DEVELOPMENT_GUIDE.md](EXTENSION_DEVELOPMENT_GUIDE.md) (Part 2: Extension development).

---

## Lifecycle hooks (Node CLI today)

1. **Create a hook module** (e.g. `hooks.mjs` in the project root):

```js
export default {
  name: "my-hook",
  beforeBuild(context) {
    console.log("Running custom step before build...");
  },
  afterBuild(context) {},
  beforeDeploy(context) {},
  afterDeploy(context) {
    console.log("Deploy finished.");
  },
};
```

2. **Reference it in `runfabric.yml`:**

```yaml
hooks:
  - ./hooks.mjs
  - ./other-hooks.mjs
```

3. **Run with the Node CLI** so hooks execute: use `npx @runfabric/cli deploy` (or the thin CLI that runs hook modules). The Go binary (`runfabric` from `make build`) does **not** run hook modules.

**Build order:** If you have multiple build steps or hooks, use `build.order` in config to define execution order.

---

## Hook API contract (v1)

Aligned with [upstream PLUGIN_API](https://github.com/runfabric/runfabric/blob/main/docs/PLUGIN_API.md). Execution of hook modules is performed by the Node/CLI layer when using the thin CLI or SDK.

**Scope (today):** `runfabric.yml` supports a top-level `hooks` array. Each hook module must export either a `default` object implementing lifecycle methods, or the module object implementing them directly. This is the **Node-side addon hook surface**, not the long-term provider/runtime/simulator plugin interface.

**Hook shape:**

```ts
import type { LifecycleHook } from "@runfabric/sdk";

const hook: LifecycleHook = {
  name: "example-hook",
  beforeBuild(context) {},
  afterBuild(context) {},
  beforeDeploy(context) {},
  afterDeploy(context) {},
};

export default hook;
```

**Callbacks:** `beforeBuild(context)`, `afterBuild(context)`, `beforeDeploy(context)`, `afterDeploy(context)`.

**Context types** (from `@runfabric/sdk`): `BuildHookContext`, `DeployHookContext`, `DeployFailure`. Hook **execution** uses the SDK: `loadHookModules(hookPaths, cwd)` and `runLifecycleHooks(hookModules, phase, context)`; the Node CLI calls these when you run `npx @runfabric/cli deploy` or build.

**Stability:**

- Existing callback names are stable in the current release train.
- Existing context field names are stable for additive evolution only.
- New optional fields may be added in minor releases.
- Removals/renames require a major release and migration notes.

**Example:**

```js
import { appendFileSync } from "node:fs";

export default {
  name: "audit-hook",
  beforeBuild() {
    appendFileSync("./hook.log", "beforeBuild\n");
  },
  afterDeploy(context) {
    appendFileSync(
      "./hook.log",
      `afterDeploy providers=${context.deployments?.length ?? 0}\n`,
    );
  },
};
```

**Testing:** Lifecycle hook contract coverage may live in the CLI or upstream repo (e.g. `tests/hooks-lifecycle.test.ts`). When changing hook loading or context fields, update tests and this doc together. The Node **SDK** does not run hooks; the **CLI** runs them.

---

See [ADDON_CONTRACT.md](ADDON_CONTRACT.md) for addon implementation details and [EXTENSION_DEVELOPMENT_GUIDE.md](EXTENSION_DEVELOPMENT_GUIDE.md) for full extension development guidance.
