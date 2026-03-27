## RunFabric Addons

Addons are **function-level or app-level augmentations** (e.g. Sentry, Datadog, OpenTelemetry, auth wrappers, env injection). They are declared in `runfabric.yml` and resolved by the engine at build/deploy time; they do **not** own providers, runtimes, or simulators.

- **Use addons for**: instrumentation, env injection, handler wrapping, generated helper files, build-time patches.
- **Do not use addons for**: provider execution, runtime packaging engines, deploy/remove/invoke/logs ownership, simulator backends.

**CLI support:** The Go binary supports addons today: config loading, validation, `runfabric extensions addons list`, and addon secret injection at deploy. (Node-side addon lifecycle hooks are covered by the addon contract and the extension development guide.)

---

## Quick navigation

- **Declare + attach**: Quickstart
- **List/catalog**: Commands + Catalog
- **Build an addon**: Implementation contract

## Quickstart (declare → attach → deploy)

1. **Declare in `runfabric.yml`** under top-level `addons`:

```yaml
addons:
  sentry:
    version: "1"
    options:
      tracesSampleRate: 1.0
    secrets:
      SENTRY_DSN: "${env:SENTRY_DSN}" # or a key into secrets:
  my-addon:
    secrets:
      MY_API_KEY: "${env:MY_ADDON_KEY}"
```

2. **Optional:** Reference a secret via the top-level `secrets` map:

```yaml
secrets:
  sentry_dsn: "${env:SENTRY_DSN}"

addons:
  sentry:
    secrets:
      SENTRY_DSN: sentry_dsn # resolves to secrets.sentry_dsn
```

3. **Per-function:** Attach only specific add-ons to a function via that function entry's `addons` field:

```yaml
functions:
  - name: api
    addons: ["sentry"]
  - name: worker
    addons: ["datadog"]
```

If `addons` is omitted on a function, all top-level addons apply.

---

## Commands

- `runfabric extensions addons list` — Shows the built-in catalog (e.g. sentry, datadog, logdrain). Set `addonCatalogUrl` in config to merge entries from a JSON URL (array of `{ "name", "version?", "description?" }`).

---

## Catalog (built-in or URL)

Extend the built-in list in the engine (e.g. `engine/internal/config/addons.go` `AddonCatalog()`) or serve a catalog JSON from a URL and set `addonCatalogUrl` in `runfabric.yml`. Add-on **behavior** is just “these env vars get injected”; the app code (or SDK) uses them (e.g. Sentry SDK reads `SENTRY_DSN`).

---

## Implementation contract (when you’re building an addon)

Addons that participate in build-time application (env, files, patches, handler wrappers, build steps) must implement the **Addon** interface: `supports({ runtime, provider })` and `apply(...)` returning **AddonResult**. See [ADDON_CONTRACT.md](../apps/registry/docs/ADDON_CONTRACT.md) for the TypeScript interface and semantics.

**Development:** For step-by-step addon development (contract, catalog, permissions, testing), see [EXTENSION_DEVELOPMENT_GUIDE.md](../apps/registry/docs/EXTENSION_DEVELOPMENT_GUIDE.md) (Part 1: Addon development).

---

## See also

- [RUNFABRIC_YML_REFERENCE.md](RUNFABRIC_YML_REFERENCE.md) (addons and `addonCatalogUrl`)
- [PLUGINS.md](../apps/registry/docs/PLUGINS.md) (plugins vs hooks)
- [EXTENSION_DEVELOPMENT_GUIDE.md](../apps/registry/docs/EXTENSION_DEVELOPMENT_GUIDE.md) (addon development)
