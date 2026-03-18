# Registry development (repo structure)

This repo now includes a scaffold for building the hosted **RunFabric Extension Registry**.

## Quick navigation

- **Registry API contract**: [EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md](EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md)
- **External plugin loading**: [EXTERNAL_EXTENSIONS_PLAN.md](EXTERNAL_EXTENSIONS_PLAN.md)
- **Registry MVP spec (API + DB schema)**: [REGISTRY_API_DB_SCHEMA_MVP_V1.md](REGISTRY_API_DB_SCHEMA_MVP_V1.md)
- **Production security + DDoS**: [REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md](REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md)

---

## Root structure

```
runfabric/
├── engine/               # RunFabric CLI + engine (Go module)
├── registry/             # Registry backend service (separate Go module)
├── web/extensions/       # Future marketplace web UI
├── schemas/              # Core schemas (runfabric.yml) + registry schemas (schemas/registry/)
```

### `engine/`

The main RunFabric CLI/engine implementation (what users run).

### `registry/`

A backend service scaffold that implements **`GET /v1/extensions/resolve`** for local contract testing.

Run locally:

```bash
cd registry
go run ./cmd/registry --listen 127.0.0.1:8787
```

## `.runfabricrc` (CLI registry config)

You can configure registry defaults per-project (similar to `.npmrc`) by adding a `.runfabricrc` file at the repo root:

```ini
registry.url=http://127.0.0.1:8787
registry.token=YOUR_BEARER_TOKEN
```

Precedence:

- CLI flags (`--registry`, `--registry-token`)
- env (`RUNFABRIC_REGISTRY_URL`, `RUNFABRIC_REGISTRY_TOKEN`)
- nearest `.runfabricrc` in the current directory ancestry (then `~/.runfabricrc`)
- built-in default (`https://registry.runfabric.cloud`)

### `web/extensions/`

Reserved for a future web UI for browsing extensions.

### `schemas/`

- `schemas/*.schema.json`: existing config schemas (e.g. `runfabric.yml`).
- `schemas/registry/`: registry API payload schemas (resolve response + version records).

