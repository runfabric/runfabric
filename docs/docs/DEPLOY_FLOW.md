# Deploy flow: CLI → app → controlplane / deployapi / deployexec

How **deployrunner**, **controlplane**, **deployapi**, **deployexec**, and **deployplan** are used and how they connect to the CLI and provider actions.

---

## 1. Entry: CLI commands

| CLI command | File | Calls |
|-------------|------|--------|
| `runfabric deploy` | `internal/cli/deploy.go` | `app.Deploy(configPath, stage)` |
| `runfabric remove` | `internal/cli/remove.go` | `app.Remove(configPath, stage)` |
| `runfabric invoke` | `internal/cli/invoke.go` | `app.Invoke(configPath, stage, function, payload)` |
| `runfabric logs` | `internal/cli/logs.go` | `app.Logs(configPath, stage, function)` |

All commands use **global options** (e.g. `opts.ConfigPath`, `opts.Stage`, `opts.JSONOutput`) and print result via `printSuccess` / `printFailure` / `printJSONSuccess`.

---

## 2. App layer: routing by provider

**Location:** `internal/app/` — `deploy.go`, `remove.go`, `invoke.go`, `logs.go`

Each app function:

1. Calls **`app.Bootstrap(configPath, stage)`** → loads config, backends, registry, root dir.
2. Reads **`ctx.Config.Provider.Name`** (e.g. `aws`, `aws-lambda`, `digitalocean-functions`, `vercel`).
3. Routes to the right path:

### Deploy (`app.Deploy`)

| Condition | Path | What runs |
|-----------|------|-----------|
| `provider == "aws"` or `"aws-lambda"` | **Control plane + deployrunner** | `controlplane.RunDeploy(..., awsprovider.NewAdapter(), ...)` |
| `deployapi.HasRunner(provider)` | **Deploy API** | `deployapi.Run(ctx, provider, cfg, stage, root)` → provider Runner in `internal/deploy/api/` |
| else | **Lifecycle stub** | `lifecycle.Deploy(registry, ...)` (simulated) |

### Remove (`app.Remove`)

| Condition | Path | What runs |
|-----------|------|-----------|
| `deployapi.HasRemover(provider)` | **Deploy API** | `deployapi.Remove(...)` → provider Remover |
| else | **Control plane + registry** | `controlplane.RunRemove(..., registry.Get(provider), ...)` (AWS real Remove; others stub) |

### Invoke / Logs

| Condition | Path |
|-----------|------|
| `deployapi.HasInvoker(provider)` / `HasLogger(provider)` | `deployapi.Invoke` / `deployapi.Logs` → provider Invoker / Logger |
| else | `lifecycle.Invoke` / `lifecycle.Logs` (stub) |

---

## 3. Control plane (`internal/controlplane/`)

Used **only for AWS** deploy (and for remove when not using deployapi).

**Role:** Lock + journal around a single deploy run.

- **`Coordinator`** — holds backends: `Locks`, `Journals`, `Receipts`; `LeaseFor`, `Heartbeat`.
- **`AcquireRunContext(ctx, service, stage, "deploy")`** — acquires lock, creates a **`transactions.Journal`** for this run, returns `RunContext{Lock, Journal}`.
- **`RunDeploy(ctx, coord, adapter, cfg, stage, root)`**:
  1. `coord.AcquireRunContext` → get lock + journal.
  2. `run.Journal.IncrementAttempt()`.
  3. **`deployrunner.Run(ctx, adapter, cfg, stage, root, run.Journal)`** ← actual deploy.
  4. On success: `run.Journal.MarkCompleted()`, `run.Journal.Delete()`.
  5. Returns `result.DeployResult`.

So: **controlplane** = orchestration (lock + journal); **deployrunner** = execution of the provider’s plan.

---

## 4. Deployrunner (`internal/deployrunner/`)

**Role:** Run whatever “plan” the provider adapter returns.

- **`Run(ctx, adapter, cfg, stage, root, journal)`**:
  1. **`plan, err := adapter.BuildPlan(ctx, cfg, stage, root, journal)`**
  2. **`res, err := plan.Execute(ctx)`**
  3. On error: `plan.Rollback(ctx)`.
  4. Returns `&RunResult{DeployResult: res}`.

So deployrunner does **not** know about phases or AWS; it only knows:

- **Adapter** (e.g. AWS) → **BuildPlan** → **Plan**
- **Plan** → **Execute** → **DeployResult**

For AWS, the adapter is **`providers/aws.Adapter`** and the plan is **`providers/aws.DeployPlan`**.

---

## 5. AWS adapter and DeployPlan (`providers/aws/`)

- **`adapter.BuildPlan(...)`** → **`NewDeployPlan(cfg, stage, root, journal)`** (same package).
- **`DeployPlan.Execute(ctx)`** (in `deploy_plan.go`):
  1. **`newResumeDependencies(ctx, root, cfg, stage, journal)`** → `ResumeDependencies` (clients, journal, receipt, …).
  2. **`newDeployEngine(cfg, stage, root, deps)`** → **`*deployexec.Engine`** (defined in `deploy_resume.go`).
  3. Builds **`deployexec.Context`** (Root, Config, Stage, Artifacts, Receipt, Outputs, Metadata).
  4. **`engine.Run(ctx, execCtx)`** ← runs all phases.
  5. Builds receipt from `execCtx`, **`state.Save(root, receipt)`**.
  6. Returns **`&providers.DeployResult{Service, Stage}`**.

So **deployplan** (AWS) = “build deps + build phase engine + run engine + save receipt”. The **phase list** and all AWS logic live in **`newDeployEngine`** in **`deploy_resume.go`**.

---

## 6. Deployexec (`internal/deployexec/`)

**Role:** Generic “run a list of phases with checkpoints”.

- **`Engine`** — `Phases []Phase`, `Journal *transactions.Journal`.
- **`Engine.Run(ctx, execCtx)`**:
  - For each phase: if checkpoint already `"done"`, skip; else `Checkpoint(name, "in_progress")` → `phase.Run(ctx, execCtx)` → `Checkpoint(name, "done")`.
- **`Phase`** — interface: `Name() string`, `Run(ctx, execCtx) error`.
- **`PhaseFunc`** — concrete phase that wraps a function.

**Checkpoints** (from `checkpoints.go`) used by AWS:

- `package_artifacts`
- `discover_state`
- `ensure_http_api`
- `ensure_lambdas`
- `ensure_routes`
- `ensure_triggers`
- `reconcile_stale`
- `save_receipt`

So **deployexec** is the **engine** that runs phases and records progress in the **journal**. It does not contain AWS-specific code; AWS **injects** the phase list in **`providers/aws/deploy_resume.go`** via **`newDeployEngine`**.

---

## 7. Deploy API (`internal/deploy/api/`)

**Role:** For **non-AWS** providers, perform deploy/remove/invoke/logs via **provider REST/SDK** (no control plane, no phase engine).

- **`Run(ctx, provider, cfg, stage, root)`** — looks up **`runners[provider]`** (e.g. `do.Runner`, `vercel.Runner`), calls `runner.Deploy(...)`, saves receipt with **`state.Save`**.
- **`Remove(...)`** — **`removers[provider]`**, loads receipt, calls remover, **`state.Delete`**.
- **`Invoke(...)`** / **`Logs(...)`** — **`invokers[provider]`** / **`loggers[provider]`**, use receipt for endpoint etc.

Provider implementations live under **`providers/<name>/`** (e.g. `providers/digitalocean/deploy.go`, `providers/vercel/remove.go`). **deployapi** only **dispatches** to them and handles receipt load/save.

---

## 8. Recovery and deploy_resume

- **`runfabric recover`** (or similar) uses **`app.Recover`** which can call **`awsprovider.ResumeDeploy(ctx, cfg, stage, root, journal)`** when resuming an AWS deploy from a saved journal file.
- **`ResumeDeploy`** (`providers/aws/deploy_resume.go`):
  - Builds a **`transactions.Journal`** from the **journal file** (e.g. from S3/local).
  - **`newResumeDependencies(..., journal)`** → same dependency struct as normal deploy.
  - **`newDeployEngine(cfg, stage, root, deps)`** → same **`*deployexec.Engine`** as **DeployPlan.Execute**.
  - **`engine.Run(ctx, execCtx)`** — same phases; **completed checkpoints are skipped** (see `completedCheckpoints` in `engine.go`).
  - Saves receipt and returns.

So **deploy_resume** reuses the **same phase engine** as the normal AWS path; the only difference is how the **journal** is created (from file vs from control plane).

---

## 9. Summary diagram

```
CLI (cmd/runfabric, internal/cli)
    deploy / remove / invoke / logs
         │
         ▼
internal/app
    Deploy / Remove / Invoke / Logs
         │
         ├── AWS deploy ──────────────────────────────────────────┐
         │       │                                                 │
         │       ▼                                                 │
         │   controlplane.RunDeploy(coord, awsAdapter, ...)       │
         │       │                                                 │
         │       ▼                                                 │
         │   deployrunner.Run(ctx, adapter, cfg, stage, root, journal)
         │       │                                                 │
         │       ├── adapter.BuildPlan()  →  aws.DeployPlan        │
         │       └── plan.Execute()                                │
         │               │                                         │
         │               ▼                                         │
         │           providers/aws/deploy_plan.Execute()            │
         │               │                                         │
         │               ├── newResumeDependencies(...)            │
         │               ├── newDeployEngine(...)  →  *deployexec.Engine
         │               └── engine.Run(ctx, execCtx)              │
         │                       │                                 │
         │                       ▼                                 │
         │               deployexec.Engine (phases in deploy_resume.go)
         │                   package_artifacts → discover_state →
         │                   ensure_http_api → ensure_lambdas →
         │                   ensure_routes → ensure_triggers →
         │                   reconcile_stale                     │
         │                                                       │
         ├── Other providers (deployapi.HasRunner etc.) ──────────┤
         │       │                                                │
         │       ▼                                                │
         │   internal/deploy/api.Run | Remove | Invoke | Logs     │
         │       │                                                │
         │       └── providers/<name>.Runner / Remover / Invoker / Logger
         │                                                       │
         └── Fallback (lifecycle stub) ─────────────────────────┘
```

---

## 10. Folder and file reference

| Concept | Location | Purpose |
|--------|----------|---------|
| **controlplane** | `internal/controlplane/` | Lock + journal; `RunDeploy` / `RunRemove`; coordinator. |
| **deployrunner** | `internal/deployrunner/runner.go` | `Run(adapter, ...)` → BuildPlan → Plan.Execute. |
| **deployapi** | `internal/deploy/api/` | Run/Remove/Invoke/Logs for non-AWS via provider Runner/Remover/Invoker/Logger. |
| **deployexec** | `internal/deployexec/` | Engine + Phase + Context + checkpoints; runs a list of phases with journal. |
| **deployplan** (AWS) | `providers/aws/deploy_plan.go` | Plan that builds deps + engine and runs `engine.Run`; saves receipt. |
| Phase list (AWS) | `providers/aws/deploy_resume.go` | `newDeployEngine` builds `*deployexec.Engine` with all phases. |
| Recovery | `providers/aws/deploy_resume.go` `ResumeDeploy` | Same engine as deploy, journal from file; used from `app.Recover`. |
