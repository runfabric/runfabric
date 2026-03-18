# Repository layout

## Quick navigation

- **High-level tree**: diagram below
- **What to edit for X**: Notes section

This is the current on-disk layout of the repo (high level):

```
runfabric/
├── engine/                 # Go CLI + core implementation
│   ├── cmd/runfabric/       # main CLI entrypoint
│   ├── internal/            # app/cli/config/planner/providers/state/runtime/etc.
│   ├── providers/           # provider adapters (aws, gcp, azure, cloudflare, ...)
│   ├── runtimes/            # language runtimes/build helpers
│   ├── test/                # engine tests (unit/integration)
│   ├── go.mod
│   └── go.sum
│
├── packages/               # SDKs / language packages (published artifacts)
│   ├── node/                # @runfabric/* (cli, sdk, providers, etc.)
│   ├── python/
│   ├── go/
│   ├── java/
│   └── dotnet/
│
├── schemas/                # JSON schemas (runfabric.yml, resources, workflows, protocol)
├── examples/               # runnable example configs/projects (grouped by runtime)
├── docs/                   # product + contributor documentation
├── scripts/                # release/dev scripts
├── .github/                # CI workflows, templates
├── bin/                    # built binaries (e.g. `bin/runfabric`)
├── Makefile
└── README.md
```

Notes:

- **Build**: `make build` builds `engine/cmd/runfabric` into `bin/runfabric`.
- **Go CLI**: the authoritative CLI is implemented in `engine/`.
- **Node CLI wrapper**: `packages/node/cli` (when present) can invoke the Go binary.
- **Examples**: see `examples/README.md` and `docs/QUICKSTART.md`.
