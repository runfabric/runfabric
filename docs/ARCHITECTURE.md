# Architecture

In this repo the Go engine lives under **`engine/`**; paths below refer to `engine/internal/`, `engine/providers/`, etc.

---

## Deploy flow: CLI → app → controlplane / deployapi / deployexec

How **deployrunner**, **controlplane**, **deployapi**, **deployexec**, and **deployplan** connect to the CLI and provider actions.

### 1. Entry: CLI commands

| CLI command        | File                     | Calls                                              |
| ------------------ | ------------------------ | -------------------------------------------------- |
| `runfabric deploy` | `internal/cli/deploy.go` | `app.Deploy(configPath, stage)`                    |
| `runfabric remove` | `internal/cli/remove.go` | `app.Remove(configPath, stage)`                    |
| `runfabric invoke` | `internal/cli/invoke.go` | `app.Invoke(configPath, stage, function, payload)` |
| `runfabric logs`   | `internal/cli/logs.go`   | `app.Logs(configPath, stage, function)`            |

### 2. App layer: routing by provider

**Location:** `internal/app/` — `deploy.go`, `remove.go`, `invoke.go`, `logs.go`

Each app function calls **`app.Bootstrap(configPath, stage)`**, reads **`ctx.Config.Provider.Name`**, and routes:

- **Deploy:** `provider == "aws"` → controlplane + deployrunner; else **deployapi.HasRunner(provider)** → deployapi.Run; else lifecycle stub.
- **Remove:** deployapi.HasRemover → deployapi.Remove; else controlplane + registry.
- **Invoke / Logs:** deployapi.HasInvoker / HasLogger → deployapi; else lifecycle stub.

### 3. Control plane (`internal/controlplane/`)

Used only for **AWS** deploy/remove. Lock + journal; **RunDeploy** → **deployrunner.Run** with AWS adapter.

### 4. Deployrunner (`internal/deployrunner/`)

**Run(ctx, adapter, cfg, stage, root, journal)** → adapter.BuildPlan → plan.Execute; on error plan.Rollback.

### 5. AWS adapter and DeployPlan (`providers/aws/`)

**adapter.BuildPlan** → **NewDeployPlan**; **DeployPlan.Execute** builds **deployexec.Engine** (phases in deploy_resume.go), runs engine, saves receipt.

### 6. Deployexec (`internal/deployexec/`)

Generic phase engine: list of phases with checkpoints; journal records progress. AWS injects phase list in **providers/aws/deploy_resume.go**.

### 7. Deploy API (`internal/deploy/api/`)

For **non-AWS** providers: deploy/remove/invoke/logs via provider REST/SDK. Dispatches to **providers/<name>/**; handles receipt load/save.

### 8. Recovery and deploy_resume

**runfabric recover** can call **awsprovider.ResumeDeploy** with journal from file; same phase engine, completed checkpoints skipped.

---

## Provider code layout

Provider code lives under **`providers/<name>/`**:

1. **Segregated actions** – deploy, remove, invoke, logs (in `deploy.go`, `remove.go`, `invoke.go`, `logs.go` or `api_*.go`). Orchestration in `internal/deploy/api/` or control plane for AWS.
2. **Resources and triggers** – each provider has **`resources/`** and **`triggers/`** per capability matrix (`internal/planner/capability_matrix.go`).

### Structure

```
providers/
├── aws/          # deploy_plan.go, deploy_resume.go, triggers/, resources/
├── cloudflare/   # api_*.go, triggers/
├── vercel/       # deploy, remove, invoke, logs, triggers/
├── netlify/      # ...
├── fly/          # ...
├── gcp/          # ...
├── azure/        # ...
├── kubernetes/   # api_*.go, triggers/
├── alibaba/      # ...
├── digitalocean/ # ...
└── ibm/          # ...
```

### Shared helpers

- **`internal/apiutil/`** – HTTP and result helpers (Env, APIGet, APIPost, BuildDeployResult, etc.).
- **`internal/deploy/api/`** – Run/Remove/Invoke/Logs dispatch to providers; no provider-specific logic.

### Migrated providers (API-based)

| Provider               | Deploy | Remove | Invoke | Logs | Location                  |
| ---------------------- | ------ | ------ | ------ | ---- | ------------------------- |
| digitalocean-functions | ✅     | ✅     | ✅     | ✅   | `providers/digitalocean/` |
| cloudflare-workers     | ✅     | ✅     | ✅     | ✅   | `providers/cloudflare/`   |
| vercel                 | ✅     | ✅     | ✅     | ✅   | `providers/vercel/`       |
| netlify                | ✅     | ✅     | ✅     | ✅   | `providers/netlify/`      |
| fly-machines           | ✅     | ✅     | ✅     | ✅   | `providers/fly/`          |
| gcp-functions          | ✅     | ✅     | ✅     | ✅   | `providers/gcp/`          |
| azure-functions        | ✅     | ✅     | ✅     | ✅   | `providers/azure/`        |
| kubernetes             | ✅     | ✅     | ✅     | ✅   | `providers/kubernetes/`   |
| ibm-openwhisk          | ✅     | ✅     | ✅     | ✅   | `providers/ibm/`          |
| alibaba-fc             | ✅     | ✅     | ✅     | ✅   | `providers/alibaba/`      |
