# Trigger Implementation Status

This doc states what **trigger-specific code** exists for each cloud provider beyond the [Trigger Capability Matrix](EXAMPLES_MATRIX.md) (which only defines **which** triggers each provider supports, Y/N).

## Summary

| Layer | What exists |
|-------|-------------|
| **Capability matrix** | ✅ All 11 providers × 8 triggers in `internal/planner/capability_matrix.go`; used for validation. |
| **Config schema** | ✅ `EventConfig` includes **http**, **cron**, **queue**, **storage**, **eventbridge**, **pubsub**, **kafka**, **rabbitmq** (`internal/config/types.go`). |
| **Planner trigger layer** | ✅ `internal/planner/triggers.go`: `ExtractTriggers`, `TriggerKindFromEvent`, `ValidateTriggersForProvider`, `ResourceTypeForTrigger`. All providers use this for validation and plan actions. |
| **AWS** | ✅ **HTTP** fully implemented (API Gateway + routes). **Cron/queue/storage/eventbridge**: trigger code in `providers/aws/triggers/` (cron validates; queue/storage/eventbridge are TODOs). Deploy phase `CheckpointEnsureTriggers` runs all four. |
| **Other providers** | ✅ Trigger stub modules exist: `providers/<name>/triggers.go` (or `triggers/`) with `EnsureHTTP`, `EnsureCron`, etc. per capability matrix. Implementations are no-op stubs for later. |

So: **there is no code implementation for the full list of triggers for all cloud providers.** The matrix is implemented (validation/support flags); deploy/plan logic per trigger exists only for **HTTP on AWS**.

---

## By provider

### aws-lambda

- **http**: ✅ Implemented. API Gateway HTTP API, routes, integrations, function URL (`providers/aws/apigw_http.go`, `deploy_resume.go`).
- **cron**: ❌ Not implemented. Config has `EventConfig.Cron` but no deploy code (no EventBridge Scheduler / CloudWatch Events wiring).
- **queue**: ❌ Not implemented. No SQS/SNS wiring in deploy.
- **storage**: ❌ Not implemented. No S3 event wiring in deploy.
- **eventbridge**: ❌ Not implemented. `providers/aws/resources/eventbridge.go` is a stub (empty package).
- **pubsub**: N in matrix (not supported).

### gcp-functions, azure-functions, kubernetes, cloudflare-workers, vercel, netlify, alibaba-fc, digitalocean-functions, fly-machines, ibm-openwhisk

- No trigger-specific deploy/plan code. These use:
  - **Stub provider** (`internal/providers/stub.go`) for lifecycle (doctor, plan, deploy, remove, invoke, logs) with simulated deploy and matrix-based validation only, or
  - Empty/minimal adapter files under `providers/<name>/` (e.g. `resources/cron.go`, `resources/queues.go` are placeholders).

---

## What would be needed for full trigger implementation

1. **Config**: Extend `EventConfig` (or add a trigger block) for queue, storage, eventbridge, pubsub (and optionally kafka, rabbitmq) so `runfabric.yml` can describe these triggers.
2. **AWS**: Add deploy/plan logic for cron (EventBridge Scheduler or CloudWatch Events), queue (SQS), storage (S3 events), eventbridge (EventBridge rules).
3. **Other providers**: Implement real Provider (and optionally deployrunner.Adapter) per provider with trigger-specific resources (e.g. GCP Pub/Sub, Azure Queue, Cloudflare Queues, K8s CronJob) according to the capability matrix.

Until then, the **Trigger Capability Matrix** is the single source of truth for **allowed** triggers per provider; only **HTTP on AWS** has actual deploy implementation.
