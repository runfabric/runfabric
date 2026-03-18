# Extension Registry Implementation Guide (v1)

This document is an **implementation guide** for the hosted **RunFabric Extension Registry** (a marketplace backend) and its CDN artifacts. It is written from the perspective of what the **RunFabric CLI** must be able to rely on, with **security and integrity first-class**.

It complements:

- [EXTERNAL_EXTENSIONS_PLAN.md](EXTERNAL_EXTENSIONS_PLAN.md) — how plugins are discovered/loaded on disk.
- [EXTENSION_DEVELOPMENT_GUIDE.md](EXTENSION_DEVELOPMENT_GUIDE.md) — how addons/plugins are authored.
- **Repo layout scaffolding**: `registry/`, `web/extensions/`, `schemas/registry/` (see project root).

---

## Quick navigation

- **The one endpoint the CLI needs**: Resolve endpoint (`/v1/extensions/resolve`)
- **Response contract**: Resolve response shape (addon vs plugin)
- **Artifacts**: CDN layout and required metadata (checksum, signature)
- **Security**: trust model, keys, SBOM/provenance
- **Production security**: [REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md](REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md)
- **MVP spec (API + DB schema)**: [REGISTRY_API_DB_SCHEMA_MVP_V1.md](REGISTRY_API_DB_SCHEMA_MVP_V1.md)
- **CLI install algorithm**: exactly what the CLI does
- **Suggested minimal schema set**: JSON schema IDs and versioning

---

## Domains (production)

Recommended separation:

- **`runfabric.cloud`**: single frontend for **docs + marketplace UI**
- **`registry.runfabric.cloud`**: registry API (**resolve/search/publish/auth**)
- **`cdn.runfabric.cloud`**: immutable artifact delivery (**bytes only**)

## 1. Core principle: “Resolve” returns a complete install decision

The registry must provide a **single resolve call** that returns everything the CLI needs:

- which **version** to install,
- which **artifact** to download (OS/arch for binaries),
- **checksum** (required),
- **signature** (strongly recommended; required for “official/verified”),
- **manifest/config schema URLs** (optional but recommended),
- install **path hints** (so the CLI can place files deterministically).

No second API call should be required for install.

---

## 2. Resolve endpoint (required)

### 2.1 Request

**HTTP**

- **Method**: `GET`
- **Path**: `/v1/extensions/resolve`
- **Query params**:
  - `id` (required): extension ID (e.g. `sentry`, `provider-aws`)
  - `core` (required): RunFabric core/CLI version (e.g. `0.9.0`)
  - `os` (required): `darwin|linux|windows`
  - `arch` (required): `arm64|amd64`
  - `version` (optional): pin an exact version (e.g. `1.2.0`)

**Example**

`GET https://registry.runfabric.cloud/v1/extensions/resolve?id=sentry&core=0.9.0&os=darwin&arch=arm64`

### 2.2 Response envelope (required)

The response is always a single object:

- `request`: echo of the normalized request inputs.
- `resolved`: the final version record (either **addon** or **plugin**).
- `meta`: debugging/trace metadata.

**Top-level shape**

```json
{
  "request": { "id": "…", "core": "…", "os": "…", "arch": "…" },
  "resolved": { /* addon or plugin version record */ },
  "meta": { "resolvedAt": "…", "registryVersion": "v1", "requestId": "…" }
}
```

### 2.2.1 Error response standard (required)

Every error response must use **one strict shape**:

```json
{
  "error": {
    "code": "EXTENSION_NOT_FOUND",
    "message": "Extension 'sentry' was not found",
    "details": { "id": "sentry" },
    "hint": "Check the extension ID or search available extensions",
    "docsUrl": "https://runfabric.cloud/docs/extensions/search",
    "requestId": "req_..."
  }
}
```

Rules:

- **No stack traces** and no internal implementation details.
- `code` is stable and machine-readable (SCREAMING_SNAKE_CASE).
- `message` is short and human-readable.
- `requestId` is required for support/debugging.

Status mapping (minimum):

- **400** → `INVALID_REQUEST`
- **401** → `UNAUTHORIZED`
- **403** → `FORBIDDEN`
- **404** → `EXTENSION_NOT_FOUND` (or `VERSION_NOT_FOUND`)
- **409** → `VERSION_ALREADY_EXISTS`
- **422** → `VALIDATION_FAILED`
- **429** → `RATE_LIMITED`
- **500** → `INTERNAL_ERROR`

Schema reference:

- `schemas/registry/error.v1.schema.json`

### 2.3 `resolved` variant: Addon

Use when `resolved.type == "addon"`.

Required fields:

- `id`, `name`, `type:"addon"`, `version`
- `publisher` (trust metadata)
- `compatibility` (at least `core`)
- `permissions` (string list)
- `artifact` (downloadable archive: url + checksum + size; signature recommended)
- `manifest` (urls; optional)
- `integrity` (SBOM/provenance urls; optional but recommended)
- `install` (path hint; optional postInstall list)

### 2.4 `resolved` variant: Plugin

Use when `resolved.type == "plugin"`.

Required fields:

- `id`, `name`, `type:"plugin"`, `pluginKind:"provider|runtime|simulator"`, `version`
- `publisher`
- `compatibility` (at least `core`)
- `capabilities` (list)
- `artifact` (**binary** for the requested `os/arch`): url + checksum + size; signature recommended
- `manifest` / `integrity` / `install` as above (install should include `binary` name and/or path hint)

---

## 3. Artifact and CDN expectations

### 3.1 Artifacts are immutable

Once a version is published, the artifact URL content must be **immutable**:

- checksums and signatures are only useful if the bytes never change.
- “yank” a version (release status) instead of mutating.

### 3.2 Recommended CDN layout

The registry response may point anywhere, but having a stable CDN layout makes ops and caching easy:

- **Addons**: `.../extensions/addons/<id>/<version>/addon.tgz`
- **Plugins**: `.../extensions/plugins/<pluginKind>/<id>/<version>/<os>-<arch>/<binary>`

### 3.3 Required integrity fields

At minimum for every artifact:

- `checksum.algorithm == "sha256"`
- `checksum.value` is a 64-hex sha256
- `sizeBytes` present

Strongly recommended:

- `signature.algorithm == "ed25519"`
- `signature.publicKeyId` present

---

## 4. Security model (minimum viable)

### 4.1 Trust levels

The registry should expose publisher trust as a user-facing concept:

- `official`: RunFabric-owned publisher + keys
- `verified`: verified org/user + keys
- `community`: unsigned or self-signed is allowed, but should be clearly indicated

### 4.2 Key distribution

The CLI needs a way to obtain trusted public keys:

- **Option A (simple v1)**: ship official public keys in the CLI binary and only verify `publicKeyId` against those.
- **Option B**: add a registry endpoint `GET /v1/keys/<publicKeyId>` (cacheable) and pin via TOFU or trust policy.

### 4.3 SBOM and provenance

Not required for v1 install, but recommended for auditing:

- `integrity.sbomUrl`
- `integrity.provenanceUrl`

---

## 5. CLI install algorithm (what the CLI does)

This is the contract your registry/CDN must satisfy.

### 5.1 Install (addons + plugins)

1. **Resolve** via `/v1/extensions/resolve`.
2. **Validate compatibility**:
   - `resolved.compatibility.core` satisfies the current CLI version.
   - for addons: optional runtime/provider constraints.
   - for plugins: ensure requested `os/arch` is supported.
3. **Download artifact** to cache.
4. **Verify checksum** (required).
5. **Verify signature** (if present and policy requires it).
6. **Extract/install**:
   - addons: install into addon cache/store (implementation-specific).
   - plugins: install into `RUNFABRIC_HOME/plugins/<kind>/<id>/<version>/...` and ensure `plugin.yaml` + executable exist.
7. **Record local receipt** (recommended): what was installed, when, what checksum/signature, and the resolve requestId for audit.

### 5.2 Uninstall / upgrade

- **Uninstall**: remove installed version dir(s).
- **Upgrade**: resolve again (without `version`), compare with installed, install newer, optionally keep N previous versions.

---

## 6. Suggested JSON Schemas (recommended)

Use JSON Schema Draft 2020-12. Keep schemas **versioned** and small.

Suggested IDs (from your `Untitled-1` direction):

- `https://registry.runfabric.cloud/schema/extension.base.v1.json`
- `https://registry.runfabric.cloud/schema/addon.version.v1.json`
- `https://registry.runfabric.cloud/schema/plugin.version.v1.json`
- `https://registry.runfabric.cloud/schema/resolve.response.v1.json`
- `https://registry.runfabric.cloud/schema/publisher.v1.json`
- `https://registry.runfabric.cloud/schema/advisory.v1.json`
- `https://registry.runfabric.cloud/schema/search.result.v1.json`

Notes:

- Keep `type` as the discriminator (`addon` vs `plugin`).
- Keep `pluginKind` only for plugin variant.
- Treat artifact objects as immutable records.

---

## 7. Practical MVP scope (what to build first)

If you want the smallest registry that the CLI can use safely:

- **Implement**: `GET /v1/extensions/resolve`
- **Host artifacts** on a CDN with immutable URLs
- **Provide sha256** for all artifacts
- **Ship official public key(s)** in the CLI and return signatures for official extensions
- **Add later**: search, advisories, publisher endpoints, publishing workflow

