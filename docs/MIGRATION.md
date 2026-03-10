# Migration Guide (Scaffold -> Real Deployments)

This guide describes how to move from current scaffold behavior to production deployment behavior.

## Current Scaffold Behavior
- `runfabric deploy` currently writes deployment receipts to `.runfabric/deploy/<provider>/deployment.json`.
- Several provider endpoints are placeholder/local values.
- Builder output is metadata-focused, not a full production packaging pipeline.

## Target Production Behavior
- Provider adapters call real provider APIs/CLIs.
- Deploy outputs contain real resource identifiers and public endpoints.
- Build pipeline emits provider-ready artifacts per runtime/provider contract.

## Migration Steps

1. Pick one provider as first production target.
2. Replace provider `deploy` stub with real API/CLI flow.
3. Replace placeholder endpoint generation with provider response parsing.
4. Expand `validate` to enforce provider-specific required config and shape.
5. Add integration tests for `plan -> build -> deploy` with deterministic fixtures.
6. Add rollback/error semantics for failed deploy operations.
7. Repeat per provider.

## Recommended First Target
- `cloudflare-workers` or `aws-lambda` are practical first provider targets due broad usage and clear deploy APIs.

## Definition Of Done For A Provider
- `doctor` catches required credentials/config.
- `build` generates provider-ready artifacts.
- `deploy` returns real endpoint + resource metadata.
- `logs` and `invoke` have real behavior (or documented provider limitation).
