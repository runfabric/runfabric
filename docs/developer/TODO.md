# Project TODO

This file tracks **open work only**. Completed items are intentionally removed so the list stays actionable.

## Quick navigation

- **Roadmap**: [ROADMAP.md](ROADMAP.md)
- **Contributor workflow**: [REPO_DEVELOPMENT.md](REPO_DEVELOPMENT.md), [BUILD_AND_RELEASE.md](BUILD_AND_RELEASE.md)

---

## Phase 15 — Extensions (in-depth working TODO)

Goal: external plugins on disk + subprocess protocol + install flow, without changing the core CLI workflow.

### Phase 15c follow-ups — subprocess lifecycle + UX hardening

- Reuse a single plugin subprocess per command context (instead of spawning per call) with an idle timeout.
- Add handshake/version negotiation (protocol + pluginVersion) and clear incompatibility errors.
- Improve failure mode UX:
  - malformed JSON / partial lines
  - non-zero exit codes (surface stderr safely)
  - timeouts and cancellation semantics
- Add optional debug logging behind a flag/env (secret-safe).
- Optional OTEL spans around plugin calls (no sensitive attributes).

### Phase 15d follow-ups — registry install security + receipts

- Verify **signatures** (ed25519) for registry installs; define policy (official/verified required, community optional).
- Persist a local install receipt for registry installs (resolved version, artifact checksum/signature, requestId).
- Improve checksum/signature mismatch errors (include requestId and actionable hint).

### Phase 15e — Registry / Marketplace (MVP v1)

This is tracked in detail in:

- [REGISTRY_API_DB_SCHEMA_MVP_V1.md](REGISTRY_API_DB_SCHEMA_MVP_V1.md)
- [REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md](REGISTRY_SECURITY_DDOS_PRODUCTION_GUIDE.md)

Open items (CLI + DX):

- Implement `runfabric extension publish` (init/upload/finalize/status).
- Add a short “External extensions quickstart” doc (install → list → use in config).

### Cross-cutting — Go SDK for external plugin authors

- Publish a Go plugin SDK (separate module) so plugin binaries can be built without importing the engine:
  - wire-compatible request/response types
  - stdio server loop helper (read line, dispatch, write line)
  - example plugin binary used by integration tests

