# Registry development (repo structure)

This repo includes the hosted **RunFabric Extension Registry** module for local development and API contract testing.

## Quick navigation

- **Registry API contract**: [EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md](EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md)
- **External plugin loading**: [EXTENSION_DEVELOPMENT_GUIDE.md](EXTENSION_DEVELOPMENT_GUIDE.md)
- **Registry MVP spec (API + DB schema)**: [REGISTRY_API_DB_SCHEMA_MVP_V1.md](REGISTRY_API_DB_SCHEMA_MVP_V1.md)
- **Production security + DDoS**: [REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md](REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md)

---

## Root structure

```
runfabric/
├── cmd/                  # binary entrypoints (runfabric, runfabricd)
├── internal/             # core CLI/app packages
├── platform/             # contracts, state, planner, runtime, extensions
├── apps/registry/        # Registry service (API + SPA in one deployable)
│   ├── internal/         # API handlers, auth, policy, metadata adapters
│   └── web/              # Registry UI (extension docs + marketplace + auth UX)
├── schemas/              # Core schemas (runfabric.yml) + registry schemas (schemas/registry/)
```

### `apps/registry/`

A service (separate Go module) that implements the registry API plus static SPA serving from the same process.
The API stack is in `apps/registry/internal/*`; the frontend is in `apps/registry/web/` and builds to `apps/registry/web/dist`.

Run locally:

```bash
cd apps/registry
npm --prefix web run build
go run ./cmd/registry --listen 127.0.0.1:8787 --web-dir ./web/dist
```

Required templates:

- `apps/registry/.env.example` for environment-based local/prod setup.
- `apps/registry/configs/config.yaml` for canonical deployment/operator config mapping.
- `apps/registry/web/` for SPA source and docs-loader build pipeline.
- Registry supports `--config <path>` (or `REGISTRY_CONFIG`) to load YAML config.
- Config precedence is `flags > env > config file > defaults`.

Production-style local run (Postgres + Redis + OIDC/JWKS):

```bash
cd apps/registry
go run ./cmd/registry \
  --listen 127.0.0.1:8787 \
  --web-dir ./web/dist \
  --ui-auth-url https://auth.runfabric.cloud/device \
  --ui-docs-url https://runfabric.cloud/docs \
  --seed-local-dev-data=true \
  --metadata-provider postgres \
  --postgres-dsn "postgres://user:pass@127.0.0.1:5432/registry?sslmode=disable" \
  --postgres-driver pgx \
  --redis-addr 127.0.0.1:6379 \
  --oidc-issuer https://auth.runfabric.cloud \
  --oidc-audience registry \
  --oidc-jwks-url https://auth.runfabric.cloud/.well-known/jwks.json \
  --casbin-policy internal/adapter/policy/casbin/policy.csv
```

Production-style local run (MongoDB + Redis + OIDC/JWKS):

```bash
cd apps/registry
go run ./cmd/registry \
  --listen 127.0.0.1:8787 \
  --web-dir ./web/dist \
  --ui-auth-url https://auth.runfabric.cloud/device \
  --ui-docs-url https://runfabric.cloud/docs \
  --metadata-provider mongodb \
  --mongodb-uri "mongodb://127.0.0.1:27017" \
  --mongodb-database "runfabric_registry" \
  --redis-addr 127.0.0.1:6379 \
  --oidc-issuer https://auth.runfabric.cloud \
  --oidc-audience registry \
  --oidc-jwks-url https://auth.runfabric.cloud/.well-known/jwks.json \
  --casbin-policy internal/adapter/policy/casbin/policy.csv
```

Implemented endpoints:

- `GET /v1/extensions/resolve`
- `GET /v1/extensions/search`
- `GET /v1/extensions/{id}`
- `GET /v1/extensions/{id}/versions`
- `GET /v1/extensions/{id}/versions/{version}`
- `GET /v1/extensions/{id}/advisories`

- `POST /v1/extensions/publish/init`
- `PUT /v1/uploads/{publishId}/{key}`
- `POST /v1/extensions/publish/finalize`
- `GET /v1/publish/{publishId}`
- `GET /packages`
- `GET /packages/{namespace}/{name}`
- `GET /packages/{namespace}/{name}/versions`
- `GET /packages/{namespace}/{name}/versions/{version}`
- `POST /packages`
- `POST /packages/{namespace}/{name}/versions`
- `PATCH /packages/{namespace}/{name}`
- `DELETE /packages/{namespace}/{name}`
- `POST /packages/{namespace}/{name}/versions/{version}/upload-url`
- `GET /packages/{namespace}/{name}/versions/{version}/download-url`

Publish/write endpoints require authentication (`Authorization: Bearer <token>` or `Authorization: ApiKey <key>`).

Auth modes supported by registry endpoints:

- `Authorization: Bearer <jwt>`
- `Authorization: ApiKey <raw_key>`
- anonymous reads for public package endpoints (when enabled)

Note: local API key fixture (`rk_local_dev`) is seeded only when `--seed-local-dev-data=true` (or `REGISTRY_SEED_LOCAL_DEV_DATA=true`) is set.

The server enforces a standard error envelope, route-aware rate limiting, and audit logging.
Audit events carry `actor_id` + `tenant_id` fields, and policy checks are tenant-aware (`subject`, `object`, `action`, `tenant`).
`GET /v1/audit` responses are tenant-scoped by caller identity.

Artifact URL mode:

- Local signed URL mode (default): package upload/download actions point to `/artifacts/...` with expiring signatures.
- S3 URL mode:
  - pass `--s3-base-url` for direct S3/S3-compatible URL mapping, or
  - pass `--s3-bucket`, `--s3-region`, `--s3-access-key-id`, `--s3-secret-access-key` (optional `--s3-endpoint`, `--s3-session-token`) to return SigV4 presigned URLs.

Postgres migrations:

- SQL migration file: `apps/registry/internal/adapter/repository/postgres/migrations/001_init.sql`
- Migrations are applied automatically on startup when `--postgres-dsn` is configured.
- Postgres driver is provided via `database/sql`; ensure your runtime registers the driver named by `--postgres-driver` (default: `pgx`).
- Metadata adapters are isolated in `apps/registry/internal/adapter/repository/{postgres,mongodb}`; `apps/registry/internal/store` uses a config-driven metadata interface boundary.
- Metadata backend selection is config-driven (`--metadata-provider auto|json|postgres|mongodb`) with parity tests across JSON/Postgres/MongoDB paths.
- Auth identity is strict for tenant safety: authenticated calls must resolve both `subject` and `tenant_id`, and JWT tenant source is `tenant_id` claim only.
- Tenant-bound policy subjects do not fall back to token roles for unbound tenants.

Integration test (optional, requires a reachable Postgres DSN):

```bash
cd apps/registry
REGISTRY_TEST_POSTGRES_DSN="postgres://user:pass@127.0.0.1:5432/registry?sslmode=disable" \
REGISTRY_TEST_POSTGRES_DRIVER=pgx \
go test ./internal/adapter/repository/postgres -run TestRepositoryIntegration_PostgresLifecycle -v
```

Metadata parity test for MongoDB path (optional, requires reachable MongoDB URI):

```bash
cd apps/registry
REGISTRY_TEST_MONGODB_URI="mongodb://127.0.0.1:27017" \
REGISTRY_TEST_MONGODB_DATABASE="runfabric_registry_test" \
go test ./internal/store -run TestMetadataParity_MongoDB -v
```

## `.runfabricrc` (CLI registry config)

You can configure registry defaults per-project (similar to `.npmrc`) by adding a `.runfabricrc` file at the repo root:

```ini
registry.url=http://127.0.0.1:8787
registry.token=YOUR_BEARER_TOKEN
auth.url=http://127.0.0.1:8787
```

Precedence:

- CLI flags (`--registry`, `--registry-token`)
- env (`RUNFABRIC_REGISTRY_URL`, `RUNFABRIC_REGISTRY_TOKEN`)
- nearest `.runfabricrc` in the current directory ancestry (then `~/.runfabricrc`)
- built-in default (`https://registry.runfabric.cloud`)

For auth/device-code commands, auth URL precedence is:

- CLI flag (`--auth-url`)
- env (`RUNFABRIC_AUTH_URL`)
- `.runfabricrc` (`auth.url`)
- fallback to `.runfabricrc` `registry.url`
- built-in default (`https://auth.runfabric.cloud`)

## Publish flow (CLI)

Example against local registry:

```bash
cat > .runfabricrc <<'EOF'
registry.url=http://127.0.0.1:8787
registry.token=local-dev-token
EOF

runfabric extensions publish init acme-provider --version 1.0.0 --artifact ./dist/provider.zip
runfabric extensions publish upload --publish-id <publishId>
runfabric extensions publish finalize --publish-id <publishId>
runfabric extensions publish status --publish-id <publishId>
```

### `apps/registry/web/`

Registry-centric frontend app (static SPA):

- route surface: `/`, `/docs`, `/docs/[...slug]`, `/extensions`, `/extensions/[id]`, `/extensions/[id]/versions/[version]`, `/publishers/[publisher]`, `/search`, `/auth`
- docs loader: `apps/registry/web/lib/docs` maps extension-dev docs from `docs/developer` into `docs-index.json`
- marketplace pages consume live registry endpoints (`/v1/extensions/*`, `/packages*`)
- build output: `apps/registry/web/dist` (served by registry server with cache-friendly asset headers)
- auth redirect is config-driven via `server.ui_auth_url` / `REGISTRY_UI_AUTH_URL` (`/v1/ui/config` for SPA consumption)
- CLI docs link in UI footer is config-driven via `server.ui_docs_url` / `REGISTRY_UI_DOCS_URL`

Docker compose option:

```bash
docker compose -f infra/docker-compose.registry.yml up -d --build
docker compose -f infra/docker-compose.registry.yml down
```

### `schemas/`

- `schemas/*.schema.json`: existing config schemas (e.g. `runfabric.yml`).
- `schemas/registry/`: registry API payload schemas (resolve response + version records).
