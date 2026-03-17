# Add-ons

Add-ons are declarative integrations (e.g. Sentry, Datadog) declared in `runfabric.yml`. At deploy, their **secrets** are resolved and injected into the function environment. No custom code runs at deploy time—only config and env binding.

**Go CLI support:** The Go binary fully supports add-ons: config loading, validation, `runfabric addons list`, and addon secret injection at deploy. Use the Go CLI or Node CLI.

---

## How to add one

1. **Declare in `runfabric.yml`** under top-level `addons`:

```yaml
addons:
  sentry:
    version: "1"
    options:
      tracesSampleRate: 1.0
    secrets:
      SENTRY_DSN: "${env:SENTRY_DSN}"   # or a key into secrets:
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
      SENTRY_DSN: sentry_dsn   # resolves to secrets.sentry_dsn
```

3. **Per-function:** Attach only specific add-ons to a function via `functions.<name>.addons`:

```yaml
functions:
  api:
    addons: ["sentry"]
  worker:
    addons: ["datadog"]
```

If `addons` is omitted on a function, all top-level addons apply.

---

## Listing add-ons

`runfabric addons list` shows the built-in catalog (sentry, datadog, logdrain, custom). Set `addonCatalogUrl` in config to merge entries from a JSON URL (array of `{ "name", "version?", "description?" }`).

---

## Adding a new add-on to the catalog

Extend the built-in list in the engine (e.g. `engine/internal/config/addons.go` `AddonCatalog()`) or serve a catalog JSON from a URL and set `addonCatalogUrl` in `runfabric.yml`. Add-on **behavior** is just “these env vars get injected”; the app code (or SDK) uses them (e.g. Sentry SDK reads `SENTRY_DSN`).

---

See also: [RUNFABRIC_YML_REFERENCE.md](RUNFABRIC_YML_REFERENCE.md) (add-ons and `addonCatalogUrl`), [PLUGINS.md](PLUGINS.md) (lifecycle hooks).
