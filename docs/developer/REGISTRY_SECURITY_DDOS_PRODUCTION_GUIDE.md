# RunFabric Registry & CDN Security + DDoS Protection (Production Guide)

This is a practical security blueprint for protecting:

- **`registry.runfabric.cloud`** (control plane: trust + metadata + auth)
- **`cdn.runfabric.cloud`** (data plane: artifact delivery)

It complements the API contract in:

- [EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md](EXTENSION_REGISTRY_IMPLEMENTATION_GUIDE.md)

---

## 1. Core security model (do not break this)

```
Registry → decides trust (metadata, versions, signatures)
CDN      → serves bytes (artifacts)
CLI      → verifies integrity (checksum + signature)
```

If you mix these responsibilities, the system becomes insecure.

---

## 2. Threat model

### Registry API risks

- account takeover
- malicious publish
- namespace hijacking
- DB exhaustion (DDoS)
- token abuse
- privilege escalation

### CDN risks

- bandwidth abuse
- artifact tampering
- cache poisoning
- origin bypass
- cost explosion

---

## 3. Architecture (minimum required)

```
User CLI
   ↓
Cloudflare / Edge (DDoS protection)
   ↓
Registry API (auth + metadata)
   ↓
Object Storage (private)
   ↓
CDN (public distribution)
```

---

## 4. Registry API protection

### 4.1 Authentication

- Short-lived access tokens
- API tokens for CLI
- MFA for publishers/admins
- Token scopes (read / publish / admin)

### 4.2 Authorization (RBAC)

Roles:

- anonymous (read only)
- publisher
- org maintainer
- reviewer
- admin

Strict rule: **publisher can only publish to owned namespace**.

### 4.3 Rate limiting (critical)

Public endpoints:

- `/resolve` → 100–300 req/min/IP
- `/search` → 50–100 req/min/IP

Auth:

- `/auth/login` → 5–10 req/min/IP

Publish:

- `/publish/init` → 10 req/min/token
- `/publish/finalize` → 10 req/min/token

Also enforce both:

- per-IP limits
- per-token limits

### 4.4 Input validation

- strict JSON schema validation
- reject unknown fields
- limit payload size
- validate semver
- sanitize query params

Never rely on CLI validation.

### 4.5 Audit logging

Log:

- publish attempts
- failed auth
- namespace changes
- key rotation
- revocations

### 4.6 Caching (huge performance gain)

- `/resolve` → 30–120s
- `/search` → 10–30s

This reduces DDoS impact significantly.

### 4.7 DB protection

- connection pool limits
- query timeouts
- indexed queries only
- avoid full scans
- read replicas if needed

---

## 5. CDN protection

### 5.1 Immutable artifacts (mandatory)

Good:

```
/extensions/plugins/aws/1.0.0/linux-amd64/...
```

Bad:

```
/extensions/aws/latest
```

### 5.2 No public writes

- uploads only via signed URLs
- registry controls uploads
- storage is private

### 5.3 Strong caching

- versioned artifacts → long TTL (hours/days)
- metadata → short TTL

### 5.4 Protect origin

- private bucket
- CDN-only access
- block direct access

### 5.5 Abuse control

- rate limit downloads per IP
- detect repeated downloads
- block abusive IPs

---

## 6. Supply chain security

### 6.1 Checksums

- SHA256 for every artifact

### 6.2 Signatures

- Ed25519 signatures
- verify in CLI

### 6.3 Publisher keys

- key rotation supported
- revoke compromised keys
- track key usage

### 6.4 Provenance (for official packages)

- SBOM
- build metadata

---

## 7. Publishing security

Required controls:

- namespace ownership enforced
- immutable versions (no overwrite)
- server-side validation
- malware scanning
- permission declaration required

Publish flow:

1. validate manifest
2. sign artifact
3. upload via signed URL
4. registry verifies
5. publish or reject

---

## 8. DDoS protection strategy

### 8.1 Edge protection (mandatory)

Use:

- Cloudflare / Fastly / AWS Shield

### 8.2 Rate limiting

This is the most important defense (see §4.3).

### 8.3 Adaptive protection

During attack:

- reduce rate limits
- enable challenge mode
- block regions if needed

### 8.4 CLI traffic identification

Require a clear User-Agent, e.g.:

```
User-Agent: runfabric-cli/1.x
```

Then:

- treat CLI differently
- detect fake clients

### 8.5 Kill switch

When under attack:

- **Registry**: disable publish endpoints; serve cached responses
- **CDN**: increase cache TTL; block IP ranges

---

## 9. Secrets management

- use a secret manager (not env files in repo)
- rotate keys regularly
- separate auth secrets from upload signing secrets
- redact secrets in logs

---

## 10. Monitoring & alerts

Track:

- request rate
- error rate
- latency
- DB load
- cache hit ratio
- unusual IP patterns

Alert on:

- traffic spikes
- repeated failed auth
- publish abuse
- signature failures

---

## 11. Error handling (security-safe)

Use the standard format:

```json
{
  "error": {
    "code": "EXTENSION_NOT_FOUND",
    "message": "Extension not found",
    "requestId": "req_xxx"
  }
}
```

Rules:

- no stack traces
- no internal details
- include `requestId` always

---

## 12. Local CLI security

The CLI must:

1. download → temp dir
2. verify checksum
3. verify signature
4. only then install

Never execute before verification.

---

## 13. Minimal production setup (do this first)

If you’re starting:

1. Put everything behind Cloudflare (or equivalent)
2. Add rate limiting (IP + token)
3. Cache `/resolve`
4. Use immutable artifact paths
5. Protect storage origin
6. Verify checksum + signature in CLI

---

## 14. What not to do

- No rate limiting
- Trust CDN blindly
- Allow version overwrite
- No signature verification
- Expose storage publicly
- One admin token for everything
- No audit logs

---

## Summary

| Layer | Responsibility |
|------:|----------------|
| Registry | trust, metadata, auth |
| CDN | fast immutable delivery |
| CLI | verification & enforcement |

