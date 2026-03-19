# RunFabric Extension Registry (API + UI)

This folder is the starting point for the **RunFabric Extension Registry** service described in:

- [docs/developer/EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md](../docs/developer/EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md)
- [docs/developer/REGISTRY_BACKEND_FINAL_IMPLEMENTATION_MAP.md](../docs/developer/REGISTRY_BACKEND_FINAL_IMPLEMENTATION_MAP.md)

## What this service is

The registry is the **marketplace service** that answers a single, complete install decision and serves the registry web UI:

- `GET /v1/extensions/resolve?id=...&core=...&os=...&arch=...`

It tells the CLI exactly which version to install and where to download it, including integrity metadata (checksum/signature).
It also serves a static SPA for extension docs + marketplace pages from the same process (`--web-dir`).

## What exists

- HTTP server in `cmd/registry/` using the Go standard library.
- JSON DB-backed store (`internal/store`) with deterministic compatibility selection for resolve.
- Optional Postgres metadata mode for package/auth/audit records (`--postgres-dsn`) with automatic migrations.
- Optional MongoDB metadata mode for package/auth/audit records (`--mongodb-uri`) with automatic index setup.
- Config-driven metadata backend selection via `--metadata-provider` (`auto|json|postgres|mongodb`).
- Optional local-dev fixture seeding via `--seed-local-dev-data` (disabled by default).
- Optional Redis read cache (`--redis-addr`) with graceful fallback to in-memory/database reads.
- Metadata adapters are isolated in `internal/adapter/repository/{postgres,mongodb}` and consumed via the store metadata interface.
- Frontend app in `web/` with docs loader under `web/lib/docs`; build output `web/dist`.
- API handlers (`internal/server`) with:
  - standardized error envelope (`code`, `message`, `details`, `hint`, `docsUrl`, `requestId`),
  - request IDs,
  - route-aware rate limiting,
  - audit event recording (`actor_id`, `tenant_id`),
  - tenant-aware policy enforcement (`subject`, `object`, `action`, `tenant`).
- Registry endpoints:
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
- Package endpoints:
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
- Artifact endpoint:
  - `PUT /artifacts/{artifact_key}?method=PUT&exp=...&sig=...`
  - `GET /artifacts/{artifact_key}?method=GET&exp=...&sig=...`
- Signature/checksum policy in publish/resolve flows:
  - publish uploads verify declared `sha256`,
  - verified publishers require signed plugin artifacts (`ed25519` local-dev key in local mode).

Auth modes:

- `Authorization: Bearer <jwt>` (supports local-dev bearer and claim-based JWT parsing)
- `Authorization: ApiKey <raw_key>` (hash lookup; local key `rk_local_dev` exists only when local-dev fixture seeding is enabled)
- anonymous reads for public package endpoints (`GET /packages*`) when enabled
- authenticated identities require both `sub` and `tenant_id` context (JWT claim for bearer, stored tenant for API key)

OIDC/JWT verification:

- If `--oidc-jwks-url` is set, Bearer JWT signatures are verified with that JWKS endpoint.
- If `--oidc-jwks-url` is not set and `--oidc-issuer` is set, the registry auto-discovers JWKS via `/.well-known/openid-configuration`.
- Optional `--oidc-issuer` and `--oidc-audience` enforce `iss`/`aud` claims.
- Audience matching modes are configurable: `exact` (default), `includes`, or `skip`.
- JWT algorithm verification is allowlist-driven (`--oidc-allowed-jwt-algs`, default `RS256,ES256`).
- `exp` and `nbf` claims are validated when present.
- Identity claim mapping is configurable (`subject`, `tenant`, `roles`) including namespaced claim keys.
- Role extraction precedence is configurable with modes: `roles`, `realm_access.roles`, `resource_access.<client>.roles`, `scope`.

Tenant isolation behavior:

- package reads/writes enforce `tenant_id` + `visibility` boundaries.
- RBAC bindings are tenant-aware; if a subject is explicitly tenant-bound in policy, unbound tenants are denied by default.
- `GET /v1/audit` is tenant-scoped and returns events only for the caller tenant.

## Run locally

From this directory:

```bash
npm --prefix web run build
go run ./cmd/registry --listen 127.0.0.1:8787 --web-dir ./web/dist
```

Config templates:

- `.env` template: `registry/.env.example`
- YAML config map: `registry/configs/config.yaml`

Configuration precedence:

- CLI flags
- environment variables
- `--config` (or `REGISTRY_CONFIG`) YAML file
- built-in defaults

Example using env file:

```bash
cd registry
cp .env.example .env
set -a
source .env
set +a
go run ./cmd/registry \
  --listen "${REGISTRY_LISTEN}" \
  --web-dir "${REGISTRY_WEB_DIR}" \
  --ui-auth-url "${REGISTRY_UI_AUTH_URL}" \
  --ui-docs-url "${REGISTRY_UI_DOCS_URL}" \
  --db "${REGISTRY_DB_PATH}" \
  --uploads "${REGISTRY_UPLOADS_DIR}" \
  --metadata-provider "${REGISTRY_METADATA_PROVIDER}" \
  --seed-local-dev-data="${REGISTRY_SEED_LOCAL_DEV_DATA}" \
  --postgres-dsn "${REGISTRY_POSTGRES_DSN}" \
  --postgres-driver "${REGISTRY_POSTGRES_DRIVER}" \
  --mongodb-uri "${REGISTRY_MONGODB_URI}" \
  --mongodb-database "${REGISTRY_MONGODB_DATABASE}" \
  --redis-addr "${REGISTRY_REDIS_ADDR}" \
  --allow-anonymous-read="${REGISTRY_ALLOW_ANONYMOUS_READ}" \
  --artifact-signing-secret "${REGISTRY_ARTIFACT_SIGNING_SECRET}" \
  --oidc-issuer "${REGISTRY_OIDC_ISSUER}" \
  --oidc-audience "${REGISTRY_OIDC_AUDIENCE}" \
  --oidc-jwks-url "${REGISTRY_OIDC_JWKS_URL}" \
  --oidc-subject-claim "${REGISTRY_OIDC_SUBJECT_CLAIM}" \
  --oidc-tenant-claim "${REGISTRY_OIDC_TENANT_CLAIM}" \
  --oidc-roles-claim "${REGISTRY_OIDC_ROLES_CLAIM}" \
  --oidc-role-modes "${REGISTRY_OIDC_ROLE_MODES}" \
  --oidc-role-client-id "${REGISTRY_OIDC_ROLE_CLIENT_ID}" \
  --oidc-audience-mode "${REGISTRY_OIDC_AUDIENCE_MODE}" \
  --oidc-allowed-jwt-algs "${REGISTRY_OIDC_ALLOWED_JWT_ALGS}" \
  --casbin-policy "${REGISTRY_CASBIN_POLICY}" \
  --s3-base-url "${REGISTRY_S3_BASE_URL}" \
  --s3-bucket "${REGISTRY_S3_BUCKET}" \
  --s3-region "${REGISTRY_S3_REGION}" \
  --s3-endpoint "${REGISTRY_S3_ENDPOINT}" \
  --s3-access-key-id "${REGISTRY_S3_ACCESS_KEY_ID}" \
  --s3-secret-access-key "${REGISTRY_S3_SECRET_ACCESS_KEY}" \
  --s3-session-token "${REGISTRY_S3_SESSION_TOKEN}"
```

Example using YAML config directly:

```bash
cd registry
go run ./cmd/registry --config ./configs/config.yaml
```

Then:

```bash
curl "http://127.0.0.1:8787/v1/extensions/resolve?id=sentry&core=0.9.0&os=darwin&arch=arm64"
```

Optional flags:

```bash
go run ./cmd/registry --listen 127.0.0.1:8787 --web-dir ./web/dist --db ./data/registry.db.json --uploads ./data/uploads
```

Enable local-dev fixtures (for example local API key `rk_local_dev`):

```bash
go run ./cmd/registry --seed-local-dev-data=true
```

With explicit auth/read controls:

```bash
REGISTRY_ARTIFACT_SIGNING_SECRET=dev-secret \
go run ./cmd/registry --listen 127.0.0.1:8787 --web-dir ./web/dist --allow-anonymous-read=true
```

## Docker (single container: API + SPA)

From repo root:

```bash
docker build -t runfabric-registry:latest -f registry/Dockerfile .
docker run --rm -p 8787:8787 runfabric-registry:latest
```

The image defaults `REGISTRY_WEB_DIR=/app/web/dist`, so `/` serves the registry SPA and API routes remain under `/v1/*`, `/packages*`, and `/artifacts/*`.

Compose stack (recommended for repeatable local runs):

```bash
docker compose -f infra/docker-compose.registry.yml up -d --build
docker compose -f infra/docker-compose.registry.yml down
```

If you previously started `runfabric-registry` via `make docker-registry-run`, run `make docker-registry-stop` once before switching to compose mode.

Postgres + Redis + OIDC example:

```bash
go run ./cmd/registry \
  --listen 127.0.0.1:8787 \
  --metadata-provider postgres \
  --postgres-dsn "postgres://user:pass@127.0.0.1:5432/registry?sslmode=disable" \
  --postgres-driver pgx \
  --redis-addr 127.0.0.1:6379 \
  --oidc-issuer https://auth.runfabric.cloud \
  --oidc-audience registry \
  --oidc-audience-mode exact \
  --oidc-allowed-jwt-algs RS256,ES256 \
  --oidc-role-modes roles,realm_access.roles,resource_access.<client>.roles,scope \
  --oidc-role-client-id registry \
  --casbin-policy internal/adapter/policy/casbin/policy.csv
```

MongoDB + Redis + OIDC example:

```bash
go run ./cmd/registry \
  --listen 127.0.0.1:8787 \
  --metadata-provider mongodb \
  --mongodb-uri "mongodb://127.0.0.1:27017" \
  --mongodb-database "runfabric_registry" \
  --redis-addr 127.0.0.1:6379 \
  --oidc-issuer https://auth.runfabric.cloud \
  --oidc-audience registry \
  --oidc-audience-mode exact \
  --oidc-allowed-jwt-algs RS256,ES256 \
  --oidc-role-modes roles,realm_access.roles,resource_access.<client>.roles,scope \
  --oidc-role-client-id registry \
  --casbin-policy internal/adapter/policy/casbin/policy.csv
```

## OIDC provider compatibility presets

### Keycloak (realm roles + resource roles, discovery)

```yaml
auth:
  oidc:
    issuer: "https://keycloak.example.com/realms/runfabric"
    audience: "registry-api"
    audience_mode: "exact"
    subject_claim: "sub"
    tenant_claim: "tenant_id"
    roles_claim: "roles"
    role_modes: "resource_access.<client>.roles,realm_access.roles,scope"
    role_client_id: "registry-api"
    allowed_jwt_algs: "RS256,ES256"
```

### Auth0 (namespaced custom claims)

```yaml
auth:
  oidc:
    issuer: "https://tenant.us.auth0.com/"
    audience: "https://registry.runfabric.cloud"
    audience_mode: "exact"
    subject_claim: "sub"
    tenant_claim: "https://runfabric.cloud/tenant_id"
    roles_claim: "https://runfabric.cloud/roles"
    role_modes: "roles,scope"
    allowed_jwt_algs: "RS256,ES256"
```

### Generic OIDC (explicit JWKS override)

```yaml
auth:
  oidc:
    issuer: "https://auth.internal.example.com"
    jwks_url: "https://auth.internal.example.com/custom/jwks.json"
    audience: "registry"
    audience_mode: "includes"
    subject_claim: "sub"
    tenant_claim: "tenant_id"
    roles_claim: "roles"
    role_modes: "roles,scope"
    allowed_jwt_algs: "RS256"
```

Note: the registry module uses `database/sql` and does not bundle a Postgres driver. Use a build/runtime environment that registers the driver name you pass to `--postgres-driver` (default `pgx`).

S3 URL mode for artifact actions:

- Use `--s3-base-url` (for example `https://registry-artifacts.s3.ap-southeast-1.amazonaws.com`) to make package upload/download URL actions return S3-style URLs.
- For SigV4 presigned URLs, provide `--s3-bucket`, `--s3-region`, `--s3-access-key-id`, and `--s3-secret-access-key` (optional `--s3-endpoint`, `--s3-session-token`).

## Testing

Run unit and handler tests:

```bash
cd registry
go test ./...
```

Optional Postgres integration test (runs only when DSN is provided):

```bash
cd registry
REGISTRY_TEST_POSTGRES_DSN="postgres://user:pass@127.0.0.1:5432/registry?sslmode=disable" \
REGISTRY_TEST_POSTGRES_DRIVER=pgx \
go test ./internal/adapter/repository/postgres -run TestRepositoryIntegration_PostgresLifecycle -v
```

Optional MongoDB parity test (runs only when URI is provided):

```bash
cd registry
REGISTRY_TEST_MONGODB_URI="mongodb://127.0.0.1:27017" \
REGISTRY_TEST_MONGODB_DATABASE="runfabric_registry_test" \
go test ./internal/store -run TestMetadataParity_MongoDB -v
```
