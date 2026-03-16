# Repository layout

Matches the structure in `cli/Untitled`:

```
runfabric/
├── engine/                              # Go core engine
│   ├── cmd/
│   │   ├── runfabric/           # main CLI binary
│   │   └── runfabric-devd/              # local dev daemon (optional)
│   ├── internal/                       # config, protocol, planner, builder, deploy, state, devserver, diagnostics
│   ├── providers/                     # aws_lambda, cloudflare_workers, gcp_functions, azure_functions, kubernetes, common
│   ├── simulators/                     # local cloud emulation (aws_lambda, cloudflare_workers, http, common)
│   ├── runtimes/                       # language runtime execution
│   │   ├── node/                       # runner.go, dev_runner.go, package_builder.go
│   │   ├── python/                     # runner.go, venv.go, package_builder.go
│   │   └── common/                     # runtime.go, env.go
│   └── go.mod
│
├── packages/                   # Per-runtime CLI and SDK (see docs/FILE_STRUCTURE.md)
│   ├── node/
│   │   ├── cli/                # @runfabric/cli (bin/)
│   │   └── sdk/                # @runfabric/sdk (adapters)
│   ├── python/runfabric/       # runfabric (CLI + SDK)
│   ├── go/sdk/                 # Go SDK (handler/)
│   ├── java/sdk/               # io.runfabric:runfabric-sdk
│   └── dotnet/sdk/             # RunFabric.Sdk
│
├── schemas/
│   ├── runfabric.schema.json   # main config schema (runfabric.yml)
│   ├── resource.schema.json
│   ├── workflow.schema.json
│   ├── secrets.schema.json
│   └── protocol/
│
├── examples/
│   ├── node/                   # Node/TS examples (hello-aws, hello-http, etc.)
│   ├── python/
│   ├── go/
│   ├── java/
│   └── dotnet/
│
├── docs/
│   ├── architecture/
│   ├── framework-guides/
│   └── providers/
│
├── Makefile                            # build from engine/, output bin/runfabric
└── README.md
```

- **Build**: `make build` builds `engine/cmd/runfabric` into `bin/runfabric`.
- **CLI**: Go binary from `engine/`; npm package **@runfabric/cli** at `packages/node/cli` invokes the binary.
- **Packages** under `packages/`: **Node** `packages/node/cli` (@runfabric/cli), `packages/node/sdk` (@runfabric/sdk), **Python** `packages/python/runfabric`, **Go** `packages/go/sdk`, **Java** `packages/java/sdk`, **.NET** `packages/dotnet/sdk`.

## Layout spec (reference)

```
runfabric/
├─ engine/                    # Go
│  ├─ cmd/
│  ├─ internal/
│  ├─ providers/
│  ├─ simulators/
│  ├─ runtimes/
│  ├─ go.mod
│  └─ go.sum
│
├─ packages/                 # node/cli, node/sdk, python/runfabric, go/sdk, java/sdk, dotnet/sdk
│
├─ schemas/
├─ examples/ (node/, python/, go/, java/, dotnet/)
├─ docs/
├─ scripts/
├─ .github/
├─ Makefile
└─ README.md
```
