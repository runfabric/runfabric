# Project TODO

Only pending work is listed here. Completed items are removed.

## P0 - AWS Event And IAM Capability Parity

- Finalize and lock the schema in `docs/RUNFABRIC_YML_SCHEMA_PROPOSAL.md`.
- Extend config/core types with exact fields:
  - `triggers[].type: "queue"` + `triggers[].queue` + `triggers[].batchSize` + `triggers[].maximumBatchingWindowSeconds` + `triggers[].maximumConcurrency` + `triggers[].enabled` + `triggers[].functionResponseType`
  - `triggers[].type: "storage"` + `triggers[].bucket` + `triggers[].events[]` + `triggers[].prefix` + `triggers[].suffix` + `triggers[].existingBucket`
  - `functions[].env` (`Record<string, string>`)
  - `extensions.aws-lambda.iam.role.statements[]` with:
    - `sid`
    - `effect`
    - `actions[]`
    - `resources[]`
    - `condition`
- Update parser (`packages/planner/src/parse-config.ts`) to support nested object/array fields above and validate required field sets per trigger type.
- Update planner validation (`packages/planner/src/planner.ts`) with explicit rules:
  - `queue` trigger requires `queue`
  - `storage` trigger requires `bucket` and at least one `events[]`
  - `storage` requires provider capability `storageEvent`
- Implement aws-lambda deploy wiring (`packages/provider-aws-lambda/src/provider.ts`) for:
  - SQS event source mapping from `queue` trigger fields
  - S3 bucket notification from `storage` trigger fields
  - IAM role statement application from `extensions.aws-lambda.iam.role.statements[]`
  - env merge (`env` + `functions[].env`)
- Add tests:
  - parser tests for each new field path and invalid-shape failures
  - planner tests for required-field/capability errors
  - aws provider tests for mapping payload generation (SQS, S3, IAM, env)
- Add docs examples using exact fields in:
  - `docs/HANDLER_SCENARIOS.md`
  - `docs/QUICKSTART.md`

## P1 - Real Deploy Validation

- Add provider command fixtures/smoke checks for real deploy mode command parsing (`RUNFABRIC_*_DEPLOY_CMD`) in CI-friendly test coverage.
- Add provider adapter contract tests that verify:
  - expected env passed to deploy/destroy command execution
  - graceful failures when command output is invalid JSON
  - endpoint extraction behavior per provider.

## P2 - Remote State Backend And Locking

- Add `state` config schema in `runfabric.yml`:
  - `state.backend: local | postgres | s3 | gcs | azblob`
  - `state.keyPrefix` (default: `runfabric/state`)
  - `state.lock.enabled` (default: `true`)
  - `state.lock.timeoutSeconds` (default: `30`)
  - `state.local.dir` (optional override for local backend root)
  - `state.postgres.connectionStringEnv`, `state.postgres.schema`, `state.postgres.table`
  - `state.s3.bucket`, `state.s3.region`, `state.s3.keyPrefix`, `state.s3.useLockfile`
  - `state.gcs.bucket`, `state.gcs.prefix`
  - `state.azblob.container`, `state.azblob.prefix`
- Keep `local` as default backend for backward compatibility.
- Add state backend factory and decouple CLI from `LocalFileStateBackend` direct construction:
  - use selected backend in `deploy`, `remove`, and `compose` workflows.
- Expand state backend contract for remote operations:
  - add delete support for provider state record cleanup
  - add clear lock metadata for diagnostics.
- Implement Terraform-style locking semantics:
  - lock on write paths
  - fail fast on lock contention with actionable message
  - add `runfabric state force-unlock --service <name> --stage <name> --provider <name>`.
- Add state migration command:
  - `runfabric state migrate --from local --to <backend>`
  - verify checksum/record count before switching backend.
- Add state schema versioning and migration safety:
  - include `schemaVersion` in remote state records
  - add migrator path for state format upgrades and downgrade guardrails.
- Add state security controls:
  - define encryption expectations per backend (at-rest + in-transit)
  - ensure deploy `details` payload excludes/redacts secrets and credential-like values.
- Harden lock protocol:
  - lock ownership token/id on acquire
  - stale lock detection/TTL behavior
  - optional lock heartbeat/renewal for long-running deploys.
- Add operational state commands:
  - `runfabric state pull`
  - `runfabric state list`
  - `runfabric state backup`
  - `runfabric state restore`
  - `runfabric state reconcile` (drift check against provider receipts/resources).
- Define transactional state semantics:
  - write deployment lifecycle status (`in_progress`, `applied`, `failed`)
  - idempotent retry behavior for interrupted deploy/remove workflows.
- Add backend auth/IAM docs:
  - minimum permissions for postgres, s3, gcs, and azblob backends
  - least-privilege policy examples.
- Add tests:
  - backend selection/config parsing
  - lock contention behavior
  - deploy/remove with remote backend
  - force-unlock and migration flows
  - e2e CI coverage with at least one DB backend and one object-store backend.

## P3 - Release Readiness

- Replace `release-notes/0.1.0.md.sig` placeholder (`UNSIGNED`) with a real signature using `RELEASE_NOTES_SIGNING_KEY`.
- Run first non-dry-run release workflow execution and confirm:
  - npm publish order succeeds,
  - git tag is pushed,
  - GitHub release is created from `release-notes/<version>.md`.
- Ensure release is gated on P0/P1/P2 passing in CI (syntax, tests, typecheck, release checks).

## P4 - Node-First Hardening And Event Expansion

- Keep beta scope explicitly Node-first:
  - document `runtime: nodejs` as the only production-ready runtime in current release train
  - add planner validation/warnings for non-node runtimes until adapters ship.
- Harden Node build/runtime behavior:
  - deterministic TS/JS entry resolution for CJS/ESM projects
  - predictable artifact output structure and handler bootstrap behavior
  - stronger `call-local` parity with deployed runtime shape (including watch-mode stability).
- Expand trigger model beyond `http|cron|queue|storage` with concrete schemas:
  - `type: eventbridge` (`pattern`, `bus`)
  - `type: pubsub` (`topic`, `subscription?`)
  - `type: kafka` (`brokers[]`, `topic`, `groupId`)
  - `type: rabbitmq` (`queue`, `exchange?`, `routingKey?`)
- Add planner portability diagnostics for new trigger types across provider capability matrix.

## P5 - Workflow, Resources, And Secrets

- Add workflow schema:
  - `workflows[].name`
  - `workflows[].steps[]` (`function`, `next?`, `retry?`, `timeoutSeconds?`)
- Add provider orchestration abstraction for workflow deploy/invoke (start with aws-lambda + Step Functions style adapter surface).
- Extend resource schema to provision basic dependencies:
  - `resources.queues[]`
  - `resources.buckets[]`
  - `resources.topics[]`
  - `resources.databases[]`
- Persist provisioned resource identifiers in state:
  - `.runfabric/state/<service>/<stage>/<provider>.state.json` under resource address map.
- Add secrets schema and resolution contract:
  - `secrets.<KEY>: secret://<ref>`
  - provider-specific secret materialization during deploy (without writing plaintext secret values to state).

## P6 - Observability And Advanced DX

- Add traces/metrics provider contracts and CLI commands:
  - `runfabric traces --provider <name>`
  - `runfabric metrics --provider <name>`
- Add correlation metadata linking deploy receipt, invoke, and logs.
- Implement `runfabric dev` local dev loop:
  - watch rebuild
  - HTTP and queue/storage event simulation presets
  - graceful shutdown/port conflict handling
- Add CI templates/docs for:
  - preview environments per PR
  - production promotion flow (`dev -> staging -> prod`)
  - provider credential wiring examples per environment.

## P7 - Multi-Runtime Support (Post Node-First GA)

- Add typed runtime support matrix in core/planner:
  - `runtime: nodejs | python | go | java | rust | dotnet`
  - provider runtime mapping validation errors (clear per-provider unsupported runtime message).
- Extend build adapters beyond Node.js:
  - python packaging adapter (`pip`/venv artifact flow)
  - go build adapter
  - java/jar packaging adapter
  - rust binary packaging adapter
  - dotnet publish adapter

## P8 - Optional IaC Resource Provisioning (Terraform / Pulumi)

- Add optional Terraform-backed provisioning mode for resources:
  - `resources.provisioner: native | terraform | pulumi`
  - `resources.terraform.dir`
  - `resources.terraform.workspace`
  - `resources.terraform.vars`
  - `resources.terraform.autoApprove` (default `false`)
- Add optional Pulumi-backed provisioning mode for resources:
  - `resources.pulumi.project`
  - `resources.pulumi.stack`
  - `resources.pulumi.workDir`
  - `resources.pulumi.config`
  - `resources.pulumi.refresh` (default `true`)
- Add CLI wrappers for Terraform resource lifecycle:
  - `runfabric resources plan`
  - `runfabric resources apply`
  - `runfabric resources destroy`
- Add CLI wrappers for Pulumi resource lifecycle:
  - `runfabric resources preview`
  - `runfabric resources up`
  - `runfabric resources destroy`
- Define state ownership boundaries to avoid dual source-of-truth:
  - Terraform state is canonical for infra resources when `resources.provisioner=terraform`
  - Pulumi state is canonical for infra resources when `resources.provisioner=pulumi`
  - runfabric state remains canonical for function deploy metadata/endpoints
  - persist Terraform/Pulumi outputs/imported references into runfabric state without copying full backend state files.
