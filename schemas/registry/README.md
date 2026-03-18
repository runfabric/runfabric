# Registry schemas (v1) — scaffold

This folder contains JSON Schemas for the **hosted extension registry** API payloads.

These are meant to validate:

- `GET /v1/extensions/resolve` response objects
- addon/plugin version records returned under `resolved`

Authoritative contract:

- [docs/EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md](../../docs/EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md)

Note: Existing `schemas/*.schema.json` at the repo root are for **`runfabric.yml`** and core config validation.

