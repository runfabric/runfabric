# Repository development

Contributor setup for the RunFabric repo. The CLI is the Go binary; packages (CLI + SDKs) live under `packages/`. See [FILE_STRUCTURE.md](FILE_STRUCTURE.md).

## Quick navigation

- **Build/test quickly**: Local setup
- **Run the CLI**: Run CLI
- **Where things live**: Layout
- **Registry module**: [REGISTRY_DEVELOPMENT.md](REGISTRY_DEVELOPMENT.md)

## Prerequisites

- Go 1.22+
- For Node SDK work: Node.js 20+
- For Python SDK work: Python 3.9+

## Local setup

```bash
# Build local binaries
make build          # → bin/runfabric and bin/runfabricd
make test           # run tests
make release-check  # format, vet, build, test (CI gate)
```

## Run CLI

```bash
./bin/runfabric --help
./bin/runfabric doctor -c runfabric.yml
```

Or install the npm package from `packages/node/cli` (after `make build-all-platforms` and copying binaries into `packages/node/cli/bin/`) and run `npx @runfabric/cli` or link locally.

## Layout

- **engine/** – Go CLI and core (config, planner, deploy, state, providers).
- **cmd/** + **internal/** + **platform/** – Go CLI and core (config, planner, deploy, state, providers).
- **apps/registry/** – Extension registry service:
  - API backend (`apps/registry/internal/*`) for resolve/search/detail/advisories/publish APIs.
  - frontend SPA (`apps/registry/web/*`) for extension docs + marketplace + auth UX.
- **packages/** – Node CLI (`packages/node/cli`), Node SDK (`packages/node/sdk`), Python (`packages/python/runfabric`), Go (`packages/go/sdk`), Java (`packages/java/sdk`), .NET (`packages/dotnet/sdk`).
- **schemas/** – JSON schema for `runfabric.yml` and registry payload schemas (`schemas/registry/`).
- **docs/** – User and contributor documentation.

See [LAYOUT.md](LAYOUT.md) and [ARCHITECTURE.md](ARCHITECTURE.md).
