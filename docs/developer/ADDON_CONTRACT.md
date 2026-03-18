# RunFabric Addon contract (implementation interface)

Addons that participate in **build-time application** (env injection, generated files, patches, handler wrappers, build steps) must implement the following interface. This is the contract for addon modules loaded by the Node/TS layer (e.g. SDK or CLI) when building functions. The Go engine does not execute addons; it consumes addon **config** (e.g. `addons.sentry.secrets`) and may consume **AddonResult** when the build pipeline is coordinated across Node and Go.

---

## Quick navigation

- **The interface**: TypeScript interface
- **How to interpret fields**: Semantics
- **How it relates to `runfabric.yml`**: Relation to config

## TypeScript interface

```ts
export interface Addon {
  name: string
  kind: "addon"
  version: string

  supports(input: {
    runtime: string
    provider: string
  }): boolean

  apply(input: {
    functionName: string
    functionConfig: unknown
    addonConfig: unknown
    projectRoot: string
    buildDir: string
    generatedDir: string
  }): Promise<AddonResult>
}

export type AddonResult = {
  env?: Record<string, string>
  files?: Array<{ path: string; content: string }>
  patches?: Array<{ path: string; find: string; replace: string }>
  handlerWrappers?: string[]
  buildSteps?: string[]
  warnings?: string[]
}
```

---

## Semantics

- **`name`**: Addon identifier (e.g. `sentry`, `datadog`). Should match the key used in `runfabric.yml` under `addons`.
- **`kind`**: Literal `"addon"` for discrimination.
- **`version`**: Semantic version or tag of the addon implementation.

- **`supports({ runtime, provider })`**: Return `true` if this addon can be applied for the given runtime (e.g. `nodejs`, `python`) and provider (e.g. `aws-lambda`, `gcp-functions`). Used to skip addons that don’t support the current target.

- **`apply(input)`**: Run the addon for one function. Returns a promise of **AddonResult**:
  - **`env`**: Environment variables to inject into the function at build/deploy (merged with config addon secrets).
  - **`files`**: Generated files to write (path relative to project or build dir, content string).
  - **`patches`**: Text patches: for each `path`, apply find/replace (e.g. patch package.json or handler).
  - **`handlerWrappers`**: Identifiers or paths of wrapper modules to wrap the function handler (order matters).
  - **`buildSteps`**: Extra build step commands or identifiers to run before/after the main build.
  - **`warnings`**: Non-fatal messages to surface to the user.

Inputs to **`apply`**:
- **`functionName`**: The function key from `runfabric.yml` (e.g. `api`, `worker`).
- **`functionConfig`**: Resolved config for that function (runtime, handler, triggers, etc.).
- **`addonConfig`**: The entry from `addons.<key>` (options, secrets refs).
- **`projectRoot`**: Absolute path to the project root.
- **`buildDir`**: Directory for build artifacts (e.g. `.runfabric/build`).
- **`generatedDir`**: Directory for addon-generated files (e.g. `.runfabric/generated`).

---

## Relation to config

- **`runfabric.yml`** declares addons under **`addons`** and optionally **`functions.<name>.addons`**. The Go engine resolves addon **secrets** and injects them at deploy; it does not call `supports` or `apply`.
- The **Node/TS build pipeline** (SDK or thin CLI) loads addon modules, calls `supports(runtime, provider)` and, when true, calls `apply(...)`. It then merges **AddonResult** (env, files, patches, handlerWrappers, buildSteps) into the build. Resulting env is typically merged with the engine’s addon secret injection.

See also: [../user/ADDONS.md](../user/ADDONS.md) (declarative usage), [PLUGINS.md](PLUGINS.md) (plugins vs addons), [../user/RUNFABRIC_YML_REFERENCE.md](../user/RUNFABRIC_YML_REFERENCE.md) (addons section).

