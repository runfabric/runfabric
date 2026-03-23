# RunFabric Registry API Spec + DB Schema (MVP v1)

This document is a **practical, implementable spec** for the RunFabric registry system.

It covers:

- API design (public + authenticated)
- standard error format + codes
- resolve flow (CLI install)
- publish flow (CLI publish)
- Postgres DB schema + indexes + constraints
- storage/CDN layout rules
- auth model + security rules
- phased build plan

Scope:

- **Registry**: `registry.runfabric.cloud` (metadata + publish/resolve/search)
- **CDN**: `cdn.runfabric.cloud` (artifact delivery)
- **CLI**: `runfabric extensions extension install` and `runfabric extensions publish`

Out of scope (MVP v1):

- billing/monetization
- social features
- ranking/recommendations

---

## Quick navigation

- **Core concepts**: types, trust, release status
- **API conventions**: base URL, auth, request IDs
- **Errors**: one strict error envelope + mapping
- **Public endpoints**: resolve/search/summary/versions/advisories
- **Publish endpoints**: init/finalize/status/yank/deprecate
- **DB schema (Postgres)**: tables, indexes, constraints
- **Resolve logic**: filtering + selection
- **Publish flow**: CLI + server steps
- **Storage layout**: immutable artifact paths
- **Security rules**: registry/CDN/CLI responsibilities
- **Build order**: phased implementation plan

---

## 1. Core concepts

### Extension types

- `addon`
- `plugin`

### Plugin kinds (only when `type=plugin`)

- `provider`
- `runtime`
- `simulator`

### Trust levels

- `official`
- `verified`
- `community`

### Release status

- `published`
- `pending_review`
- `rejected`
- `deprecated`
- `yanked`
- `revoked`

---

## 2. High-level architecture

```text
CLI
 ↓
Registry API (resolve, search, publish)
 ↓
Signed Upload URLs / Metadata
 ↓
Object Storage (private origin)
 ↓
CDN (public immutable artifacts)
 ↓
CLI verifies checksum + signature locally
```

---

## 3. API conventions

### Base URL

`https://registry.runfabric.cloud/v1`

### Content type

`Content-Type: application/json`

### Auth

Public:

- resolve
- search
- get extension
- get versions
- advisories

Authenticated:

- publish/init
- publish/finalize
- yank/deprecate
- publisher management

Auth header:

- `Authorization: Bearer <token>`

### Request IDs (required)

All responses must include:

- header: `X-Request-Id: req_xxx`
- and in error bodies: `error.requestId`

---

## 4. Standard error response (required)

All endpoints must return **one strict error envelope**:

```json
{
  "error": {
    "code": "EXTENSION_NOT_FOUND",
    "message": "Extension 'sentry' was not found",
    "details": { "id": "sentry" },
    "hint": "Check the extension ID or search available extensions",
    "docsUrl": "https://runfabric.cloud/docs/extensions/search",
    "requestId": "req_123"
  }
}
```

Error codes (minimum set):

- `INVALID_REQUEST`
- `UNAUTHORIZED`
- `FORBIDDEN`
- `EXTENSION_NOT_FOUND`
- `VERSION_NOT_FOUND`
- `INCOMPATIBLE_EXTENSION`
- `VALIDATION_FAILED`
- `VERSION_ALREADY_EXISTS`
- `SIGNATURE_VERIFICATION_FAILED`
- `CHECKSUM_MISMATCH`
- `PUBLISH_REJECTED`
- `RATE_LIMITED`
- `INTERNAL_ERROR`

---

## 5. Public API endpoints (MVP)

### 5.1 Resolve extension (CLI install)

`GET /v1/extensions/resolve?id=sentry&core=0.9.0&os=darwin&arch=arm64`

Rules:

- `id`, `core`, `os`, `arch` are required
- registry resolves alias → canonical id
- resolve returns only **compatible** versions with `releaseStatus=published`
- errors:
  - unknown id → `404 EXTENSION_NOT_FOUND`
  - no compatible version → `422 INCOMPATIBLE_EXTENSION`

### 5.2 Search extensions

`GET /v1/extensions/search?q=sentry&type=addon&page=1&pageSize=20`

Query params:

- `q` optional
- `type` optional: `addon|plugin`
- `pluginKind` optional: `provider|runtime|simulator`
- `trust` optional
- `publisher` optional
- `page` default 1
- `pageSize` default 20, max 100

### 5.3 Get extension summary

`GET /v1/extensions/{id}`

### 5.4 List versions

`GET /v1/extensions/{id}/versions`

### 5.5 Get version details (full version payload)

`GET /v1/extensions/{id}/versions/{version}`

### 5.6 List advisories

`GET /v1/extensions/{id}/advisories`

---

## 6. Publish API endpoints (MVP)

### 6.1 Publish init (stage + signed upload URLs)

`POST /v1/extensions/publish/init`

Responsibilities:

- namespace ownership enforcement
- version uniqueness check
- light manifest validation
- issue signed upload URLs for declared files

### 6.2 Publish finalize (verify + validate + publish/reject)

`POST /v1/extensions/publish/finalize`

Finalize server-side steps:

- verify upload presence
- re-check checksums
- verify signature
- validate manifest fully (schema + policy)
- validate package structure
- malware/policy scanning
- create/update extension row
- create extension_version row
- create artifacts rows
- index for search
- set status: `published|pending_review|rejected`
- write audit log

### 6.3 Publish status

`GET /v1/publish/{publishId}`

### 6.4 Yank version

`POST /v1/extensions/{id}/versions/{version}/yank`

Rule:

- yanked versions are not returned by normal resolve
- metadata remains for history

### 6.5 Deprecate version

`POST /v1/extensions/{id}/versions/{version}/deprecate`

---

## 7. Auth and publisher model (MVP)

Token scopes:

- `registry:read`
- `registry:publish`
- `registry:manage`
- `registry:admin`

Roles:

- anonymous
- publisher
- org_maintainer
- reviewer
- admin

Rules:

- only owned namespaces can publish
- “official” packages only by official org
- “verified” controlled by moderation/admin
- all admin actions must be audited

---

## 8. Postgres DB schema (MVP v1)

This section is the recommended MVP relational shape.

### 8.1 Enum types

```sql
CREATE TYPE extension_type AS ENUM ('addon', 'plugin');
CREATE TYPE plugin_kind AS ENUM ('provider', 'runtime', 'simulator');
CREATE TYPE trust_level AS ENUM ('official', 'verified', 'community');
CREATE TYPE visibility_level AS ENUM ('public', 'private', 'unlisted');
CREATE TYPE release_status AS ENUM (
  'published',
  'pending_review',
  'rejected',
  'deprecated',
  'yanked',
  'revoked'
);
CREATE TYPE publisher_type AS ENUM ('user', 'organization');
```

### 8.2 Publishers

```sql
CREATE TABLE publishers (
  id                TEXT PRIMARY KEY,
  name              TEXT NOT NULL,
  type              publisher_type NOT NULL,
  trust             trust_level NOT NULL DEFAULT 'community',
  verified          BOOLEAN NOT NULL DEFAULT FALSE,
  homepage_url      TEXT,
  avatar_url        TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 8.3 Publisher namespaces (ownership)

```sql
CREATE TABLE publisher_namespaces (
  namespace         TEXT PRIMARY KEY,
  publisher_id      TEXT NOT NULL REFERENCES publishers(id) ON DELETE CASCADE,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Rule: extension id namespace must exist here (example namespace: `runfabric`).

### 8.4 Signing keys

```sql
CREATE TABLE signing_keys (
  id                TEXT PRIMARY KEY,
  publisher_id      TEXT NOT NULL REFERENCES publishers(id) ON DELETE CASCADE,
  algorithm         TEXT NOT NULL CHECK (algorithm IN ('ed25519')),
  public_key        TEXT NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at        TIMESTAMPTZ
);
CREATE INDEX idx_signing_keys_publisher_id ON signing_keys(publisher_id);
CREATE INDEX idx_signing_keys_active ON signing_keys(publisher_id) WHERE revoked_at IS NULL;
```

### 8.5 Extensions

```sql
CREATE TABLE extensions (
  id                TEXT PRIMARY KEY,            -- runfabric/sentry
  namespace         TEXT NOT NULL,
  slug              TEXT NOT NULL,
  name              TEXT NOT NULL,
  summary           TEXT NOT NULL,
  description       TEXT,
  type              extension_type NOT NULL,
  plugin_kind       plugin_kind,
  publisher_id      TEXT NOT NULL REFERENCES publishers(id) ON DELETE RESTRICT,
  visibility        visibility_level NOT NULL DEFAULT 'public',
  trust             trust_level NOT NULL DEFAULT 'community',
  homepage_url      TEXT,
  docs_url          TEXT,
  source_url        TEXT,
  icon_url          TEXT,
  license           TEXT,
  latest_version    TEXT,
  versions_count    INTEGER NOT NULL DEFAULT 0,
  deprecated        BOOLEAN NOT NULL DEFAULT FALSE,
  deprecated_message TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_plugin_kind_required
    CHECK (
      (type = 'plugin' AND plugin_kind IS NOT NULL)
      OR
      (type = 'addon' AND plugin_kind IS NULL)
    )
);

CREATE UNIQUE INDEX uq_extensions_namespace_slug ON extensions(namespace, slug);
CREATE INDEX idx_extensions_type ON extensions(type);
CREATE INDEX idx_extensions_plugin_kind ON extensions(plugin_kind);
CREATE INDEX idx_extensions_publisher_id ON extensions(publisher_id);
CREATE INDEX idx_extensions_updated_at ON extensions(updated_at DESC);
```

### 8.6 Categories + tags

```sql
CREATE TABLE extension_categories (
  extension_id       TEXT NOT NULL REFERENCES extensions(id) ON DELETE CASCADE,
  category           TEXT NOT NULL,
  PRIMARY KEY (extension_id, category)
);

CREATE TABLE extension_tags (
  extension_id       TEXT NOT NULL REFERENCES extensions(id) ON DELETE CASCADE,
  tag                TEXT NOT NULL,
  PRIMARY KEY (extension_id, tag)
);
```

### 8.7 Extension versions

Use structured columns for queryable fields and JSONB for the full payload.

```sql
CREATE TABLE extension_versions (
  id                 BIGSERIAL PRIMARY KEY,
  extension_id       TEXT NOT NULL REFERENCES extensions(id) ON DELETE CASCADE,
  version            TEXT NOT NULL,
  release_status     release_status NOT NULL,
  protocol_version   TEXT,               -- plugin only
  summary            TEXT NOT NULL,
  description        TEXT,
  manifest_payload   JSONB NOT NULL,     -- full addon/plugin version document
  changelog_url      TEXT,
  published_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_extension_version UNIQUE (extension_id, version)
);

CREATE INDEX idx_extension_versions_extension_id ON extension_versions(extension_id);
CREATE INDEX idx_extension_versions_status ON extension_versions(release_status);
CREATE INDEX idx_extension_versions_published_at ON extension_versions(published_at DESC);
CREATE INDEX idx_extension_versions_manifest_gin ON extension_versions USING GIN (manifest_payload);
```

### 8.8 Artifacts

One row per physical file.

```sql
CREATE TABLE artifacts (
  id                 BIGSERIAL PRIMARY KEY,
  extension_version_id BIGINT NOT NULL REFERENCES extension_versions(id) ON DELETE CASCADE,
  path               TEXT NOT NULL,              -- logical path inside release
  storage_key        TEXT NOT NULL,              -- object storage key
  artifact_role      TEXT NOT NULL,              -- addon_bundle, plugin_binary, manifest, schema, signature, sbom
  os                 TEXT,
  arch               TEXT,
  format             TEXT NOT NULL,
  binary_name        TEXT,
  size_bytes         BIGINT NOT NULL CHECK (size_bytes > 0),
  sha256             CHAR(64) NOT NULL,
  signature_algorithm TEXT,
  signature_value    TEXT,
  public_key_id      TEXT,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_artifacts_extension_version_id ON artifacts(extension_version_id);
CREATE INDEX idx_artifacts_os_arch ON artifacts(os, arch);
CREATE UNIQUE INDEX uq_artifacts_unique_path_per_version ON artifacts(extension_version_id, path);
```

### 8.9 Publish sessions + files

```sql
CREATE TABLE publish_sessions (
  id                 TEXT PRIMARY KEY,           -- pub_xxx
  publisher_id       TEXT NOT NULL REFERENCES publishers(id) ON DELETE CASCADE,
  extension_id       TEXT NOT NULL,
  version            TEXT NOT NULL,
  manifest_payload   JSONB NOT NULL,
  signature_payload  JSONB,
  status             TEXT NOT NULL CHECK (status IN ('staged', 'uploaded', 'pending_review', 'published', 'rejected', 'expired')),
  expires_at         TIMESTAMPTZ NOT NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE publish_session_files (
  id                 BIGSERIAL PRIMARY KEY,
  publish_session_id TEXT NOT NULL REFERENCES publish_sessions(id) ON DELETE CASCADE,
  path               TEXT NOT NULL,
  sha256             CHAR(64) NOT NULL,
  size_bytes         BIGINT NOT NULL CHECK (size_bytes > 0),
  uploaded           BOOLEAN NOT NULL DEFAULT FALSE,
  storage_key        TEXT,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (publish_session_id, path)
);
```

### 8.10 Advisories

```sql
CREATE TABLE advisories (
  id                 TEXT PRIMARY KEY,
  extension_id       TEXT NOT NULL REFERENCES extensions(id) ON DELETE CASCADE,
  affected_versions  TEXT NOT NULL,
  severity           TEXT NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
  summary            TEXT NOT NULL,
  details            TEXT NOT NULL,
  fixed_versions     JSONB,
  published_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_advisories_extension_id ON advisories(extension_id);
CREATE INDEX idx_advisories_severity ON advisories(severity);
```

### 8.11 Download events (optional analytics + abuse)

```sql
CREATE TABLE download_events (
  id                 BIGSERIAL PRIMARY KEY,
  extension_id       TEXT NOT NULL,
  extension_version  TEXT NOT NULL,
  artifact_id        BIGINT,
  ip_hash            TEXT,
  user_agent         TEXT,
  source             TEXT, -- cli/web
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 8.12 Audit log

```sql
CREATE TABLE audit_logs (
  id                 BIGSERIAL PRIMARY KEY,
  actor_publisher_id TEXT,
  actor_type         TEXT NOT NULL, -- user, service, admin
  action             TEXT NOT NULL, -- publish_init, publish_finalize, yank, revoke_key, etc
  target_type        TEXT NOT NULL, -- extension, version, signing_key
  target_id          TEXT NOT NULL,
  metadata           JSONB,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_publisher_id);
CREATE INDEX idx_audit_logs_target ON audit_logs(target_type, target_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
```

---

## 9. Search strategy (MVP)

Start with Postgres full-text search over:

- `extensions.name`
- `extensions.summary`
- `extensions.description`
- `extension_tags.tag`
- `extension_categories.category`

Add a generated TSV column later if needed.

---

## 10. Resolve logic (server)

Inputs:

- extension id or alias
- core version
- os
- arch

Steps:

1. normalize input id
2. find extension record
3. list candidate versions where `release_status='published'`
4. filter by compatibility:
   - core semver
   - os
   - arch
5. choose highest compatible version
6. for plugins: select matching artifact for os/arch
7. build response payload and return

Failure cases:

- unknown id → `404 EXTENSION_NOT_FOUND`
- no compatible version → `422 INCOMPATIBLE_EXTENSION`
- missing plugin artifact for os/arch → `422 INCOMPATIBLE_EXTENSION`

---

## 11. Storage layout (CDN) (immutable)

Addon layout:

```
/extensions/addons/sentry/1.2.0/
  addon.tgz
  manifest.json
  config.schema.json
  checksums.txt
  signature.sig
  sbom.spdx.json
  provenance.intoto.jsonl
```

Plugin layout:

```
/extensions/plugins/providers/aws/1.0.0/
  manifest.json
  checksums.txt
  signature.sig
  sbom.spdx.json
  provenance.intoto.jsonl
  darwin-arm64/runfabric-plugin-provider-aws
  linux-amd64/runfabric-plugin-provider-aws
  windows-amd64/runfabric-plugin-provider-aws.exe
```

Rules:

- immutable versioned paths
- no `latest`
- no overwrite

---

## 12. Security rules (non-negotiable)

Registry:

- auth + RBAC
- per-IP and per-token rate limits
- strict schema validation (reject unknown fields)
- namespace ownership enforcement
- audit logs
- signed uploads only

CDN:

- private origin
- public read via CDN only
- immutable caching for versioned files
- no direct write access

CLI:

- download to temp dir
- verify SHA256
- verify signature
- only then install

---

## 13. Build order (plan)

Phase 1:

- DB schema
- resolve endpoint
- search endpoint
- extension summary/version endpoints

Phase 2:

- publish init/finalize
- signed uploads
- artifact rows
- audit logs

Phase 3:

- signature verification policy
- advisories
- yank/deprecate
- moderation pipeline

Phase 4:

- analytics
- richer search
- verified/community workflows
