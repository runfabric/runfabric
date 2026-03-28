# Router Extension ADR

Status: Accepted (Phase 0/1 baseline)

## Purpose

Capture go/no-go constraints, contract scope, and operational safety boundaries for the router extension track.

## Decision

RunFabric adopts a **sync-first router contract** for v1:

- Required method: `Router.Sync(ctx, RouterSyncRequest) (*RouterSyncResult, error)`.
- Optional richer lifecycle methods (`plan/apply/status/rollback/inspect`) are deferred until multi-router parity exists.
- Router execution may be:
  - in-process (built-in plugins), or
  - external executable plugin (discovered and dispatched via extension boundary).

## Safety and rollout model

- Production sync requires explicit enablement (`--allow-prod-dns-sync` or config policy equivalent).
- Stage rollout gates are supported (`dev -> staging -> prod`) with approval env checks.
- Policy can require an operator reason env var before apply.
- Router sync snapshots are persisted (`.runfabric/router-sync-<stage>.json`) and can be restored with `runfabric router dns-restore`.

## Residual risks

- Provider rollback is best-effort replay of prior desired routing snapshots; exact provider-side transaction rollback is not guaranteed.
- Drift introspection includes resource-level summaries, delete-candidate preview, and sync-history trend analytics (`runfabric router dns-history` plus reconcile/restore trend output).
- Multi-provider/router conformance is pending; current operational hardening is Cloudflare-first.

## Recommendation

- Keep sync-first contract for current release window.
- Expand to explicit lifecycle interface only after:
  - at least two router providers implement identical semantics, and
  - rollback/diff/delete semantics are standardized in the contract.
