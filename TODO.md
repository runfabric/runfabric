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

