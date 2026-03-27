# Repository layout

## Quick navigation

- **High-level tree**: diagram below
- **What to edit for X**: Notes section

This is the current on-disk layout of the repo (high level):

```
runfabric/
├── apps/                   # externally-executed services (HTTP APIs, SPAs, binaries)
│   ├── cli/
│   ├── daemon/
│   ├── registry/           # extension registry service (Go API + data adapters + web)
│   │   └── .../
│
├── cmd/                    # binaries entrypoints (runfabric, runfabricd)
├── internal/               # CLI/internal engine packages
├── platform/               # provider/runtime/extensions/state modules
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
├── bin/                    # built binaries (e.g. `bin/runfabric`, `bin/runfabricd`)
├── Makefile
└── README.md
```

Notes:

- **Build**: `make build` builds `cmd/runfabric` and `cmd/runfabricd` into `bin/`.
- **Go CLI**: the authoritative CLI/runtime is implemented across `cmd/`, `internal/`, and `platform/`.
- **Node CLI wrapper**: `packages/node/cli` (when present) can invoke the Go binary.
- **Examples**: see `examples/README.md` and `docs/QUICKSTART.md`.
