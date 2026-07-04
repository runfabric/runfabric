# Provider Support Tiers & Conformance

Status: Accepted
Date: 2026-06-09

RunFabric ships 13 provider integrations, but they are not all exercised to the
same depth in CI. This document makes that explicit — which providers are
release-gating, which are best-effort — and defines the single **conformance
suite** that is the entry bar for a tier. It resolves weak point W5 in
`ARCHITECTURE_IMPROVEMENT_PLAN.md`.

## Tiers

| Tier | Meaning | Gate |
| ---- | ------- | ---- |
| **Tier 1** | Release-gating. Runs the conformance suite in CI on every release; a failure blocks the release. Sandbox/e2e runs where credentials are available. | `make conformance` (unit) + e2e-on-sandbox (when creds present) |
| **Tier 2** | Best-effort / community-supported. Expected to pass conformance; not release-gating. Regressions are fixed as reported, not blockers. | `make conformance` (unit) |

Promotion Tier 2 → Tier 1 requires: green conformance, an owner, and an e2e run
against a real account wired into CI.

## Provider matrix

Trigger columns come from `platform/planner/engine/capability_matrix.go`
(`ProviderCapabilities`) — the single source of truth kept in sync with
`docs/EXAMPLES_MATRIX.md`.

| Provider | Tier | HTTP | Cron | Queue | Storage |
| -------- | ---- | ---- | ---- | ----- | ------- |
| aws-lambda | **1** | ✓ | ✓ | ✓ | ✓ |
| gcp-functions | **1** | ✓ | ✓ | ✓ | ✓ |
| cloudflare-workers | **1** | ✓ | ✓ | – | – |
| azure-functions | 2 | ✓ | ✓ | ✓ | ✓ |
| kubernetes | 2 | ✓ | ✓ | – | – |
| vercel | 2 | ✓ | – | – | – |
| netlify | 2 | ✓ | – | – | – |
| fly-machines | 2 | ✓ | – | – | – |
| linode | 2 | ✓ | – | – | – |
| digitalocean-functions | 2 | ✓ | ✓ | – | – |
| alibaba-fc | 2 | ✓ | ✓ | ✓ | ✓ |
| ibm-openwhisk | 2 | ✓ | ✓ | – | – |
| devstream | 2 | ✓ | – | – | – |

(Trigger cells are indicative; the authoritative matrix is the Go source above.
A cell is not a conformance requirement — conformance covers the lifecycle
contract below, not every trigger.)

## The conformance contract

The suite lives in `platform/test/conformance` and drives any
`providers.ProviderPlugin` through one parameterized lifecycle, asserting the
invariants every provider must satisfy:

| Phase | Invariant |
| ----- | --------- |
| `Meta()` | `Name` is non-empty |
| `ValidateConfig` | returns no error for a valid config |
| `Plan` | returns a non-nil result |
| `Deploy` | non-nil result; `Provider` set; `DeploymentID` set (deploy is addressable) |
| `Invoke` | non-nil result; `Function` echoes the invoked function |
| `Logs` | returns a non-nil result |
| `Remove` | non-nil result; `Removed == true` |

`referenceProvider` in the suite is a minimal compliant implementation that
doubles as executable documentation: a new provider that mirrors those return
shapes passes. A `brokenProvider` test proves the suite is not vacuous — it
catches an empty `Meta` name, a nil `Deploy` result, a wrong `Invoke` echo, and
`Removed == false`.

### Running it

```sh
make conformance                     # go test ./platform/test/conformance/...
```

A provider wires itself in from its own test package:

```go
func TestMyProviderConformance(t *testing.T) {
    conformance.RunProviderConformance(t, NewMyProvider(),
        conformance.SampleConfig("my-provider"), t.TempDir(), "dev")
}
```

For an e2e run against a real account, construct the real provider (with
credentials from the environment) instead of a fake and skip when the
credentials are absent — the same opt-in pattern as the DynamoDB run-store
integration test (`RUNFABRIC_TEST_DYNAMODB_ENDPOINT`).

## Release cadence decoupling

The registry application (`apps/registry`) and the language SDKs
(`packages/{go,python,java,dotnet,node}`) do **not** need to ride the engine's
version tag:

- They have independent compatibility surfaces (the plugin protocol version and
  the registry API version), which already gate cross-version safety.
- Splitting their release trains shrinks what `make release-check` must
  guarantee per engine release to: the three binaries, the provider conformance
  suite for Tier-1, and the frozen ADR invariants.
- Concretely: tag the engine (`runfabric*`), the registry, and each SDK on their
  own cadence; `release-check` covers the engine train only.

This is a process decision recorded here; the mechanical split (separate tags /
CI workflows) is tracked separately and does not change engine code.
