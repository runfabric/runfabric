# TODO

## Deferred: Enterprise Production Readiness (Engine Track)

- [ ] Define and freeze v1 engine stability contracts.
  - Scope: CLI flags/output compatibility, run state schema compatibility, extension protocol versioning, and deprecation policy.

- [ ] Introduce a formal reliability target matrix.
  - Goal: define SLOs and error budgets for deploy, workflow execution, invoke, logs, and routing apply paths.

- [ ] Harden workflow and deploy idempotency guarantees.
  - Goal: make retry/replay behavior deterministic across controlplane, provider adapters, and state writes.

- [ ] Add persistent audit trail and provenance for critical operations.
  - Scope: deploy/remove/workflow-run/router-apply with actor identity, reason, correlation ID, and before/after metadata.

- [ ] Enforce policy gates for production actions.
  - Include: environment protection rules, required approvals, blocked dangerous flags, and policy-as-code checks.

- [ ] Implement secrets and key management integration.
  - Targets: AWS Secrets Manager, GCP Secret Manager, Vault.
  - Goal: remove static secret usage in CI and local configs for prod stages.

- [ ] Add multi-tenant safety boundaries.
  - Goal: isolate state, credentials, and execution identities by org/project/environment.

- [ ] Expand disaster recovery and backup strategy.
  - Include: state snapshots, journal restoration drills, run replay recovery tests, and documented RTO/RPO objectives.

- [ ] Build full observability for runtime and controlplane.
  - Scope: traces, metrics, logs, and audit events with standard correlation fields and dashboards.

- [ ] Add supply chain and release hardening.
  - Include: SBOM generation, signed artifacts, checksum verification, dependency vulnerability gates, and provenance attestations.

- [ ] Add scale and resilience test suites.
  - Cases: high-concurrency workflow runs, provider API throttling, network partitions, state backend latency spikes.

- [ ] Add enterprise documentation track.
  - Scope: production operations guide, security model, compliance mapping (SOC2/ISO style controls), and incident response playbooks.

- [ ] Define GA quality gates for engine releases.
  - Goal: ship only when mandatory checks, reliability thresholds, migration checks, and docs-sync gates all pass.

## Deferred: GitHub Integration for PaaS Delivery (Engine Track)

```mermaid
flowchart TD
  A[Connect GitHub Repo] --> B[Configure Source Contract\nowner/repo + branch/tag/commit + path]
  B --> C[Authenticate via GitHub App\nPAT only for local/dev fallback]
  C --> D[Fetch + Checkout by Commit SHA]
  D --> E[Build and Package]
  E --> F[Deploy to Target Environment]
  F --> G[Persist Provenance\ncommit SHA + actor + artifact digest]

  G --> H{P1 Automation Enabled?}
  H -->|Yes| I[Webhook or Manual Trigger]
  I --> J[Policy Gates\nchecks + branch rules + approvals]
  J --> K[Promote Deployment]
  H -->|No| K

  K --> L{P2 Hardening Enabled?}
  L -->|Yes| M[Rollback to Last Known Good Commit]
  L -->|Yes| N[Git Provider Conformance Suite]
  L -->|No| O[Operate with Current Controls]
```

### P0 (MVP: Connect + Pull + Deterministic Deploy)

- [ ] Define repo source contract in config and API.
  - Include: `owner/repo`, branch/tag/commit pin, path within repo, auth mode, and build context.

- [ ] Add secure GitHub auth integration.
  - Options: GitHub App installation flow (preferred), PAT fallback for local/dev only.
  - Constraint: never store raw tokens in plain config or logs.

- [ ] (Partial) Implement source fetch and checkout worker.
  - Current: archive source fetch/extract exists via deploy `--source`.
  - Remaining: first-class Git checkout by commit SHA with deterministic repository provenance.

- [ ] Add buildpack/executor contract for pulled code.
  - Goal: detect runtime and run standardized build/test/package steps before deploy.

- [ ] Persist source provenance in deployment records.
  - Include: commit SHA, actor, workflow run ID, PR number, and artifact digest.

- [ ] Add focused validation for P0.
  - `go test ./internal/cli/...`
  - `go test ./platform/deploy/...`
  - `go test ./platform/core/state/...`

### P1 (Automation: Change Triggers + Policy Gates)

- [ ] Add change detection and trigger strategy.
  - Scope: webhook ingestion (push, PR merge, release), manual sync, and scheduled poll fallback.

- [ ] Add deployment gating from Git events.
  - Include: required checks, branch policies, environment approvals, and protected production promotions.

- [ ] Add Git provider abstraction to engine deploy pipeline.
  - Goal: support GitHub first, then extendable to GitLab/Bitbucket without changing core flow.

- [ ] Add focused validation for P1.
  - `go test ./platform/deploy/...`
  - `go test ./platform/core/state/...`
  - `go test ./platform/test/...`

### P2 (Hardening: Rollback + Multi-Provider Expansion)

- [ ] Add rollback-to-commit support.
  - Goal: one-command rollback to last-known-good commit and artifact.

- [ ] Add advanced deploy provenance/audit correlation.
  - Include: cross-link deploy, run, and approval events by correlation ID.

- [ ] Add conformance suite for additional git providers.
  - Goal: ensure identical behavior and policy enforcement across GitHub/GitLab/Bitbucket integrations.

- [ ] Add focused validation for P2.
  - `go test ./internal/cli/...`
  - `go test ./platform/deploy/...`
  - `go test ./platform/core/state/...`
  - `go test ./platform/test/...`
