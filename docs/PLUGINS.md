# Plugins (lifecycle hooks)

Plugins are Node.js modules that implement lifecycle callbacks (`beforeBuild`, `afterBuild`, `beforeDeploy`, `afterDeploy`). They run when using the **Node CLI** (`@runfabric/cli`).

**Go CLI support:** The Go binary loads and validates `hooks` from config but **does not execute** Node hook modules. To use lifecycle hooks, use the Node CLI (`npx @runfabric/cli`).

---

## How to add one

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
  }
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

## Plugin API contract (v1)

Aligned with [upstream PLUGIN_API](https://github.com/runfabric/runfabric/blob/main/docs/PLUGIN_API.md). Execution of hook modules is performed by the Node/CLI layer when using the thin CLI or SDK.

**Scope:** `runfabric.yml` supports a top-level `hooks` array. Each hook module must export either a `default` object implementing lifecycle methods, or the module object implementing them directly.

**Hook shape:**

```ts
import type { LifecycleHook } from "@runfabric/sdk";

const hook: LifecycleHook = {
  name: "example-hook",
  beforeBuild(context) {},
  afterBuild(context) {},
  beforeDeploy(context) {},
  afterDeploy(context) {}
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
    appendFileSync("./hook.log", `afterDeploy providers=${context.deployments?.length ?? 0}\n`);
  }
};
```

**Testing:** Lifecycle hook contract coverage may live in the CLI or upstream repo (e.g. `tests/hooks-lifecycle.test.ts`). When changing hook loading or context fields, update tests and this doc together. The Node **SDK** does not run hooks; the **CLI** runs them.

---

See also: [RUNFABRIC_YML_REFERENCE.md](RUNFABRIC_YML_REFERENCE.md) (build order), [ADDONS.md](ADDONS.md) (declarative integrations).
