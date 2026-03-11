# Plugin And Extension API

This document defines the current stable extension surface for `runfabric`.

Contract version: `v1` (Node.js module hooks).

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

## Testing

Contract coverage is maintained in:

- `tests/hooks-lifecycle.test.ts`

When changing hook loading behavior or context fields, update tests and this doc in the same change.
