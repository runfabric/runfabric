# Controlplane / Deploy Reliability ADR

Status: Accepted
Date: 2026-06-09

## Purpose

Decide, and freeze, whether the deploy reliability pipeline (lock → journal →
phases → recovery) is an AWS-only guarantee or a provider-neutral one. This
resolves weak point W2 in `ARCHITECTURE_IMPROVEMENT_PLAN.md`.

## Context

The original W2 problem statement was that "AWS alone gets journaling, phase
execution, and `runfabric recover`, so portability silently means different
guarantees per cloud." Investigation before implementing found that premise
largely stale relative to the current tree:

- **Journaling/phases were already provider-neutral for API dispatch.**
  `platform/deploy/api/run.go` wraps every API-dispatch deploy in
  `deployexec.RunDeploy` + `OpenDeployJournal`. `deployexec.RunDeploy` had a
  single caller (the API path) — not an AWS-gated one.
- **`runfabric recover` was already capability-based.**
  `platform/workflow/app/recover.go` dispatches to any provider implementing the
  `RecoveryCapable` optional capability. That capability is implemented by
  `aws-lambda`, `gcp-functions`, and `azure-functions`.
- **The AWS-flavored `platform/deploy/controlplane` package has zero non-test
  importers.** It is an isolated, unused subsystem on the current deploy path,
  not a branch that special-cases AWS during dispatch.

The only genuine asymmetry was that plugin/internal dispatch
(`DeployLifecycle` → `lifecycle.Deploy`) ran the provider deploy **without** a
journal, while API dispatch did.

## Decision

1. **Journaled phases are the default execution wrapper for every dispatch
   mode.** Plugin/internal dispatch now runs through the same
   `deployexec.RunDeploy` + `OpenDeployJournal` engine as API dispatch
   (`platform/workflow/app/deploy.go`). Durable checkpointing, retry, and
   journal-based recovery are a uniform guarantee, not a per-provider accident.

2. **Recovery stays capability-based, not provider-hardcoded.** `runfabric
   recover` works for any provider implementing `RecoveryCapable`. Providers
   that do not implement it fail with a clear "does not support recovery"
   message rather than a silent difference in guarantees.

3. **No `ClusterCapable` interface is introduced.** There is no AWS
   special-casing in the deploy/dispatch path to demote behind a capability
   interface. `platform/deploy/controlplane` is isolated and unused; formalizing
   it as an optional capability with no implementer and no caller would add dead
   scaffolding. If a second provider ever needs cluster-orchestration behavior,
   that is the trigger to introduce the capability — following the existing
   `RecoveryCapable` / `OrchestrationCapable` / `ObservabilityCapable` pattern —
   not before.

## Consequences

- Every provider, regardless of dispatch mode, gets the same crash-and-resume
  semantics: a completed deploy leaves a `completed` journal on disk; an
  interrupted one leaves an `active` journal for retry.
- "Provider portability" now means the same reliability contract across clouds,
  bounded by whether a provider opts into the `RecoveryCapable` capability.

## Open gate (not yet satisfied)

The full-generalization change touches the deploy path for **all** providers,
but the W5 provider-conformance suite (a parameterized deploy → invoke → logs →
remove harness) does not exist yet — `platform/test/integration` is still
per-provider `aws_*_test.go` files. Confidence currently rests on the existing
integration suite plus `platform/deploy/exec` unit tests, which is narrower than
a per-provider conformance matrix. **This ADR should be revisited once W5
lands**, at which point the conformance suite becomes the regression net for the
uniform-journaling guarantee across every Tier-1 provider.

Separately, `lifecycle.Deploy` still ignores the `context.Context` its journal
phase passes it; per-step context propagation is tracked under W3 (item 1) and
is a precondition for honoring per-deploy timeouts/cancellation on this path.

## Enforcement

- `go test ./platform/deploy/exec/...` — journal completion + resume semantics,
  incl. `TestRunDeploy_PersistsCompletedJournal`.
- `go test ./platform/core/policy/architecture/...` — import-boundary rules
  (the unification must not introduce a layering violation).
- `go test ./platform/test/...` — deploy resume / transaction integration.
- `make release-check`.
