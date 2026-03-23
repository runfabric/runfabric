# Registry schemas (v1)

This folder contains JSON Schemas for the **hosted extension registry** API payloads.

These schemas validate:

- `GET /v1/extensions/resolve` response objects
- addon/plugin version records returned under `resolved`
- `GET /v1/extensions/search` result pages
- advisory records from `GET /v1/extensions/{id}/advisories`

Authoritative contract:

- [apps/registry/docs/EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md](../../apps/registry/docs/EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md)

Included:

- `resolve.response.v1.schema.json`
- `addon.version.v1.schema.json`
- `plugin.version.v1.schema.json`
- `artifact.v1.schema.json`
- `publisher.v1.schema.json`
- `error.v1.schema.json`
- `search.result.v1.schema.json`
- `advisory.v1.schema.json`

Note: Existing `schemas/*.schema.json` at the repo root are for **`runfabric.yml`** and core config validation.
