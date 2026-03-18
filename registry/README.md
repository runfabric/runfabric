# RunFabric Extension Registry (backend) — dev scaffold

This folder is the starting point for the **RunFabric Extension Registry** service described in:

- [docs/EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md](../docs/EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md)

## What this service is

The registry is the **marketplace backend** that answers a single, complete install decision:

- `GET /v1/extensions/resolve?id=...&core=...&os=...&arch=...`

It tells the CLI exactly which version to install and where to download it, including integrity metadata (checksum/signature).

## What exists in this scaffold

- A minimal HTTP server in `cmd/registry/` using the Go standard library.
- A stub `resolve` endpoint that returns a deterministic response for a small hardcoded catalog (for local development and contract testing).

## Run locally

From this directory:

```bash
go run ./cmd/registry --listen 127.0.0.1:8787
```

Then:

```bash
curl "http://127.0.0.1:8787/v1/extensions/resolve?id=sentry&core=0.9.0&os=darwin&arch=arm64"
```

## Next steps (intended)

- Replace the hardcoded catalog with a DB-backed model (publishers, extensions, versions, artifacts, keys).
- Implement signature verification policy and key distribution.
- Add `search` and advisory endpoints once `resolve` is stable.

