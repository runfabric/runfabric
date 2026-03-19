# Registry Backend Final Implementation Map

This document translates the final registry backend architecture into an executable implementation plan for this repository.

## 1. Purpose and boundary

Registry backend is a stateless resource server.

It does:

- validate auth credentials (JWT, API key, anonymous-read mode)
- enforce tenant isolation
- enforce RBAC
- manage package/version metadata
- coordinate artifact upload and download actions
- emit audit events
- apply rate limiting and abuse controls

It does not do:

- login/session/user-registration flows
- social OAuth UI flows
- tenant creation UI logic
- private app-backend business logic

## 2. Current state in this repo

Current module: `registry/` with:

- HTTP server: `registry/internal/server`
- JSON-backed store: `registry/internal/store`
- publish/resolve/search routes
- request IDs, rate limiting, audit envelope

Current gaps against final architecture:

- no Postgres metadata adapter
- no Redis cache for read paths
- no OIDC/JWKS verification
- no API-key repository and validation flow
- no tenant isolation and visibility enforcement model
- no Casbin policy engine integration
- no S3 artifact adapter

## 3. Target structure

```text
registry/
  cmd/registry/main.go
  internal/
    app/
    domain/
    ports/
    service/
    transport/http/
    adapter/
      auth/
      repository/postgres/
      storage/s3/
      policy/casbin/
      audit/postgres/
```

Keep transport, service, domain, and adapters separated. Do not spread SQL, JWT parsing, or S3 SDK calls into service/domain layers.

## 4. Core runtime model

Identity context fields:

- `subject_id`
- `tenant_id`
- `roles`
- `auth_type` (`jwt`, `api_key`, `anonymous`)
- `is_anonymous`
- `issuer`, `audience` (JWT path)

Package visibility:

- `public`: readable by all (including anonymous)
- `tenant`: readable only by owner tenant

Tenant rules:

- tenant comes only from validated JWT claims or API-key record
- never accept tenant from request body/query/header
- write/update/delete always restricted to owner tenant

## 5. Data plane and storage

Metadata:

- Postgres only (no multi-DB abstraction in V1)
- schema: `packages`, `package_versions`, `api_keys` (+ optional `audit_events`, `namespaces`)

Artifacts:

- S3 or S3-compatible object store only
- artifact key format: `tenants/{tenant_id}/packages/{namespace}/{name}/{version}/artifact.tar.gz`
- use presigned URLs for upload/download actions

Caching:

- Redis for high-traffic reads (`/packages`, detail, versions, advisories)
- cache-aside/read-through + targeted invalidation on publish/update/delete
- graceful fallback to Postgres when Redis is unavailable

## 6. Auth and authorization

Supported auth modes:

- `Authorization: Bearer <jwt>` via OIDC/JWKS verification
- `Authorization: ApiKey <raw_key>` via hash lookup in Postgres
- anonymous (public read endpoints only)

Authorization gates:

- tenant and visibility filters must pass
- Casbin RBAC must pass
- both are mandatory for protected operations

## 7. API and policy behavior

Read endpoints:

- anonymous: public packages only
- authenticated: public + tenant-owned packages

Write endpoints (`POST`, `PATCH`, `DELETE`):

- authenticated only
- tenant required
- role required (`publisher`/`admin` depending on action)
- owner tenant check required

## 8. Implementation phases

### Phase 16A — Architecture boundary and config baseline

- Introduce `internal/domain`, `internal/ports`, `internal/service`, `internal/transport/http`, and `internal/adapter/*` packages.
- Keep existing routes operational while moving business logic out of transport.
- Add config model for `auth`, `database`, `storage`, `rbac`, `audit`, and `anonymous` flags.

### Phase 16B — Auth and identity context

- Implement JWT validator adapter with issuer/audience/JWKS checks.
- Implement API key validator adapter (hash + expiry/revocation checks).
- Build auth middleware that injects a normalized identity context.
- Enforce anonymous mode only on explicit public read routes.

### Phase 16C — Tenant isolation and visibility enforcement

- Add service-level visibility and ownership rules.
- Apply repository query filters for anonymous and tenant-scoped reads.
- Block all tenant derivation from request payloads.

### Phase 16D — Postgres metadata and migrations

- Implement Postgres repositories for packages, versions, and API keys.
- Add migrations for required tables, constraints, and indexes.
- Keep SQL only in adapter/repository package.

### Phase 16E — Artifact storage with S3

- Implement object store adapter for presigned upload/download URL generation.
- Persist artifact metadata (key/checksum/size) and verify upload metadata.
- Enforce artifact path policy by tenant/package/version.

### Phase 16F — Casbin RBAC and audit

- Integrate Casbin enforcer and policy model.
- Enforce action checks (`package:read`, `package:publish`, `package:delete`, `package:manage_visibility`).
- Emit audit events for auth outcomes and sensitive package operations.

### Phase 16G — Redis cache, hardening, and performance

- Add Redis cache adapter with endpoint-specific TTLs and key versioning.
- Implement invalidation paths for publish/update/delete flows.
- Add rate-limit profiles, input sanitization, checksum validation, and immutable-version policy checks.
- Add integration/perf tests and operational metrics (cache hit ratio, DB latency, auth failures).

## 9. Done criteria

Phase 16 is complete when:

- all Phase 16A–16G items are implemented
- registry endpoints run on Postgres + S3 with Redis caching
- auth modes (JWT, API key, anonymous public read) pass integration tests
- tenant isolation and RBAC are enforced in both service and repository behavior
- audit events are emitted for all sensitive operations
- docs and runbooks are updated for local and production setup

## 10. Out of scope for this phase

- auth server internals and login UX
- advanced analytics
- multi-database support
- custom per-package ACL systems beyond baseline tenant + RBAC model
