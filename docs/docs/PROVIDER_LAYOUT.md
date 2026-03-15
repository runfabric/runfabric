# Provider code layout

Provider-related code is organised under **`providers/<name>/`** with:

1. **Segregated actions** – Each provider has its own deploy, remove, invoke, and logs (in `deploy.go`, `remove.go`, `invoke.go`, `logs.go` or `api_*.go`). Orchestration is in `internal/deploy/api/` (or control plane for AWS).
2. **Resources and triggers in separate folders per Trigger Capability Matrix** – Each provider has a **`resources/`** folder (doc + types) and a **`triggers/`** folder (trigger implementations per matrix). Aligned with `internal/planner/capability_matrix.go`.

## Structure

```
providers/
├── digitalocean/
│   ├── resources/         # doc.go: resources + triggers per capability matrix
│   ├── triggers/         # http.go, cron.go (per matrix: http, cron)
│   ├── triggers.go       # re-exports triggers.*
│   ├── deploy.go, remove.go, invoke.go, logs.go
│   └── README.md
├── cloudflare/
│   ├── resources/
│   ├── triggers/         # http.go, cron.go
│   ├── triggers.go
│   └── api_deploy.go, api_remove.go, api_invoke.go, api_logs.go
├── vercel/               # resources/, triggers/ (http, cron), triggers.go, deploy, remove, invoke, logs
├── netlify/              # resources/, triggers/ (http, cron), triggers.go, deploy, remove, invoke, logs
├── fly/                  # resources/, triggers/ (http only), triggers.go, deploy, remove, invoke, logs
├── gcp/                  # resources/, triggers/ (http, cron, queue, storage, pubsub), triggers.go, deploy, remove, invoke, logs
├── azure/                # resources/, triggers/ (http, cron, queue, storage), triggers.go, deploy, remove, invoke, logs
├── kubernetes/           # resources/, triggers/ (http, cron), triggers.go, api_*.go
├── alibaba/              # resources/, triggers/ (doc), triggers.go (impl uses fcClient), deploy, remove, invoke, logs
├── ibm/                  # resources/, triggers/ (http, cron), triggers.go, deploy, remove, invoke, logs
└── aws/
    ├── actions/          # doc.go: deploy, remove, invoke, logs entry points (impl in root)
    ├── resources/        # doc.go, lambda.go, apigw.go, iam.go, s3.go, sqs.go, eventbridge.go
    ├── triggers/         # doc.go, cron.go, queue.go, storage.go, eventbridge.go
    ├── deploy.go, remove.go, invoke.go, logs.go
    ├── deploy_resume.go, deploy_engine.go, deploy_plan.go, build.go, plan.go
    └── apigw_*.go, function_url.go, lambda.go, ...
```

## Shared helpers

- **`internal/apiutil/`** – HTTP and result helpers used by provider packages:
  - `Env(key)`, `APIGet`, `APIPost`, `APIPut`, `DoDelete`, `DefaultClient`
  - `BuildDeployResult(provider, cfg, stage)` for base `DeployResult`

## Orchestration

- **`internal/deploy/`** – Deployment paths: **`internal/deploy/api/`** (API-based deploy/remove/invoke/logs; dispatches to providers). **`internal/deploy/cli/`** (optional CLI-based deploy). Orchestration details:
  - `run.go`: `Runner` interface; registers each provider’s `Runner` (e.g. `do.Runner{}`, `cf.Runner{}`)
  - `remove.go`: `Remover` interface; registers each provider’s `Remover`
  - `invoke.go`: `Invoker` interface; registers each provider’s `Invoker`
  - `logs.go`: `Logger` interface; registers each provider’s `Logger`
  - No provider-specific logic in deploy/api; it only routes and handles receipt save/load.

## Migrated providers

| Provider               | Deploy | Remove | Invoke | Logs | Location              |
|------------------------|--------|--------|--------|------|-----------------------|
| digitalocean-functions | ✅     | ✅     | ✅     | ✅   | `providers/digitalocean/` |
| cloudflare-workers     | ✅     | ✅     | ✅     | ✅   | `providers/cloudflare/`   |
| vercel                 | ✅     | ✅     | ✅     | ✅   | `providers/vercel/`       |
| netlify                | ✅     | ✅     | ✅     | ✅   | `providers/netlify/`      |
| fly-machines           | ✅     | ✅     | ✅     | ✅   | `providers/fly/`          |
| gcp-functions          | ✅     | ✅     | ✅     | ✅   | `providers/gcp/`          |
| azure-functions        | ✅     | ✅     | ✅     | ✅   | `providers/azure/`        |
| kubernetes             | ✅     | ✅     | ✅     | ✅   | `providers/kubernetes/` (api_*.go) |
| ibm-openwhisk          | ✅     | ✅     | ✅     | ✅   | `providers/ibm/`          |
| alibaba-fc             | ✅     | ✅     | ✅     | ✅   | `providers/alibaba/` (signed FC API; triggers: http, cron, queue, storage) |

All API-based deploy/remove/invoke/logs logic lives in **`providers/<name>/`**. **`internal/deploy/api/`** only dispatches to these implementations.
