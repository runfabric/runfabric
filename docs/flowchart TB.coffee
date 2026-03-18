flowchart TB
  CLI["`cmd/runfabric`<br/>CLI entrypoint"] --> ROOT["`internal/cli/root.go`<br/>Cobra commands"]

  ROOT -->|doctor| APP_DOCTOR["`internal/app/doctor.go`"]
  ROOT -->|plan|   APP_PLAN["`internal/app/plan.go`"]
  ROOT -->|build/package| APP_BUILD["`internal/app/build.go`"]
  ROOT -->|deploy|  APP_DEPLOY["`internal/app/deploy.go`"]
  ROOT -->|remove|  APP_REMOVE["`internal/app/remove.go`"]
  ROOT -->|invoke|  APP_INVOKE["`internal/app/invoke.go`"]
  ROOT -->|logs|    APP_LOGS["`internal/app/logs.go`"]
  ROOT -->|traces|  APP_TRACES["`internal/app/traces.go`"]
  ROOT -->|metrics| APP_METRICS["`internal/app/metrics.go`"]
  ROOT -->|addons/plugin/extension| EXT_CLI["`internal/cli/addons.go` + `plugin.go` + `extension.go`"]
  ROOT -->|daemon/dashboard/config-api| SVC_CLI["`internal/cli/daemon.go` / `dashboard.go` / `config-api`"]

  subgraph BOOT["Bootstrap + config"]
    APP_BOOT["`internal/app/bootstrap.go`<br/>load/resolve/validate + registry setup + backends"] --> CFG["`internal/config/*`"]
    APP_BOOT --> REG["Provider registry<br/>`internal/extensions/providers`"]
    APP_BOOT --> MAN["Plugin manifests<br/>`internal/extensions/manifests`"]
    APP_BOOT --> BACK["Backends bundle<br/>`internal/backends/*`"]
  end

  APP_DOCTOR --> APP_BOOT
  APP_PLAN --> APP_BOOT
  APP_BUILD --> APP_BOOT
  APP_DEPLOY --> APP_BOOT
  APP_REMOVE --> APP_BOOT
  APP_INVOKE --> APP_BOOT
  APP_LOGS --> APP_BOOT
  APP_TRACES --> APP_BOOT
  APP_METRICS --> APP_BOOT

  %% Plan/Doctor (provider registry path)
  APP_DOCTOR --> LIFE_DOCTOR["`internal/lifecycle/doctor.go`"]
  APP_PLAN --> LIFE_PLAN["`internal/lifecycle/plan.go`"]
  LIFE_DOCTOR --> REG_GET["`Registry.Get(providerName)`"]
  LIFE_PLAN --> REG_GET

  %% Deploy routing
  APP_DEPLOY -->|if provider == aws/aws-lambda| CP["AWS controlplane path"]
  APP_DEPLOY -->|else if deploy/api has runner| DEP_API["API deploy path"]
  APP_DEPLOY -->|else| LIFE_DEPLOY["`internal/lifecycle/deploy.go`"]

  subgraph AWS["AWS deploy: controlplane + phase engine"]
    CP["`internal/controlplane/*`"] --> AWS_ADAPTER["AWS adapter<br/>`internal/extensions/provider/aws/adapter.go`"]
    CP --> COORD["Coordinator (locks/journals/receipts)<br/>`internal/controlplane/coordinator.go`"]
    CP --> RUNNER["`internal/deployrunner/runner.go`"]
    RUNNER --> PLAN_ENGINE["Phase engine<br/>`internal/deployexec/*`"]
    PLAN_ENGINE --> STATE["Receipts/journals/state<br/>`internal/state` + `internal/transactions` + `internal/locking`"]
  end

  subgraph API["Non-AWS deploy: provider API runners"]
    DEP_API["`internal/deploy/api/run.go`"] --> P_RUNNER["Provider Runner<br/>`internal/extensions/provider/<name>`"]
    DEP_API --> STATE
  end

  %% Remove routing
  APP_REMOVE -->|if provider == aws/aws-lambda| CP_REMOVE["`internal/controlplane` remove path"]
  APP_REMOVE -->|else if deploy/api remover| DEP_API_REMOVE["`internal/deploy/api/remove.go`"]
  APP_REMOVE -->|else| LIFE_REMOVE["`internal/lifecycle/remove.go`"]

  %% Invoke/logs routing
  APP_INVOKE -->|if deploy/api invoker| DEP_API_INVOKE["`internal/deploy/api/invoke.go`"]
  APP_INVOKE -->|else| LIFE_INVOKE["`internal/lifecycle/invoke.go`"]
  APP_LOGS -->|if deploy/api logger| DEP_API_LOGS["`internal/deploy/api/logs.go`"]
  APP_LOGS -->|else| LIFE_LOGS["`internal/lifecycle/logs.go`"]

  %% Runtime build/invoke helpers (used by providers and app build/package)
  APP_BUILD --> RT_BUILD["Runtime build helpers<br/>`internal/extensions/runtime/build/*`"]
  AWS_ADAPTER --> RT_BUILD
  P_RUNNER --> RT_BUILD
