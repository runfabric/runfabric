# Plugin And Extension API

This document defines the current stable extension surface for `runfabric`. Aligned with [upstream PLUGIN_API](https://github.com/runfabric/runfabric/blob/main/docs/PLUGIN_API.md).

Contract version: `v1` (Node.js module hooks). In this repo, `runfabric.yml` supports a top-level `hooks` array; execution of hook modules is performed by the Node/CLI layer when using the thin CLI or SDK.

## Quick navigation

- **What this covers**: Scope
- **Hook contract**: Lifecycle Contract (`v1`)
- **Compatibility**: Stability Rules
- **Go vs Node CLI behavior**: Go CLI

## Provider binary plugins

Provider plugins are standalone Go binaries that implement the plugin-sdk provider contract. They run as child processes and communicate over stdin/stdout using the plugin protocol.

### Module structure

Built-in providers (aws-lambda, gcp-functions, etc.) live inside the root Go module and are compiled into the `runfabric` binary. External provider plugins have their own `go.mod` and are installed separately.

**`extensions/providers/linode/`** is the reference implementation for an external binary plugin:
- Has its own `go.mod` with a `replace` directive pointing to `../../../packages/go/plugin-sdk`
- Imports only `github.com/runfabric/runfabric/plugin-sdk/go` — never `platform/` or `internal/`
- Built via `make build-provider-plugins` → `bin/plugins/linode-plugin`
- Registered via `plugin.yaml` after `make install-provider-plugins`

Third-party provider authors should follow this pattern: create a standalone module, import only the plugin-sdk, implement the `provider.Plugin` interface, and distribute a `plugin.yaml` alongside the binary.

## Scope

`runfabric` supports lifecycle hook modules referenced by `runfabric.yml`:

```yaml
hooks:
  - ./hooks.mjs
```

Each hook module must export either:

- `default` object implementing lifecycle methods, or
- a module object implementing lifecycle methods directly.

## Lifecycle Contract (`v1`)

Hook object shape:

```ts
import type { LifecycleHook } from "@runfabric/core";

const hook: LifecycleHook = {
  name: "example-hook",
  beforeBuild(context) {},
  afterBuild(context) {},
  beforeDeploy(context) {},
  afterDeploy(context) {}
};

export default hook;
```

Available callbacks:

- `beforeBuild(context)`
- `afterBuild(context)`
- `beforeDeploy(context)`
- `afterDeploy(context)`

Context contract types are exported from `@runfabric/core`:

- `BuildHookContext`
- `DeployHookContext`
- `DeployFailure`

## Stability Rules

- Existing callback names are treated as stable in current release train.
- Existing context field names are treated as stable for additive evolution only.
- New optional fields may be added in minor releases.
- Removals/renames require a major release and migration notes.

## Example

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

## Go CLI

When using the **Go binary** (`runfabric` built from this repo), `hooks` in `runfabric.yml` are **not executed**. The Go engine loads and validates config but does not run Node hook modules. To use lifecycle hooks (beforeBuild, afterBuild, beforeDeploy, afterDeploy), use the **Node CLI wrapper** (`@runfabric/cli`) which runs hook modules before/after the corresponding lifecycle steps.

## Testing

Contract coverage is maintained in:

- `tests/hooks-lifecycle.test.ts`

When changing hook loading behavior or context fields, update tests and this doc in the same change.
