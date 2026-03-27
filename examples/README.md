# RunFabric examples

Examples are grouped by runtime. Today this repo ships **Node/JS/TS** examples under `examples/node/`.

| Directory | Description                                                                                    |
| --------- | ---------------------------------------------------------------------------------------------- |
| **node/** | Node/JS/TS examples (hello-aws, hello-http, compose-app, compose-contracts, handler-scenarios) |

## Quick start (recommended)

From repo root:

```bash
pnpm run runfabric -- doctor -c examples/node/hello-http/runfabric.quickstart.yml
pnpm run runfabric -- plan -c examples/node/hello-http/runfabric.quickstart.yml
pnpm run runfabric -- build -c examples/node/hello-http/runfabric.quickstart.yml
pnpm run runfabric -- deploy -c examples/node/hello-http/runfabric.quickstart.yml
```

See `examples/node/hello-http/PROVIDERS.md` for provider-specific config files.

## Creating examples (via init)

To generate new examples with `runfabric init` for **js**, **ts**, **python**, and **go** (and any provider/template/state combination), see **[docs/EXAMPLES_ANALYSIS.md](../docs/EXAMPLES_ANALYSIS.md)**. That doc:

- Summarizes the [runfabric-example](https://github.com/runfabric/runfabric-example) repo (init commands, provider × template matrix).
- Gives the exact `runfabric init` pattern and examples for each language.
- Uses naming like `runfabric-<provider>-<trigger>-state-<backend>[-<lang>]`.

See also [docs/QUICKSTART.md](../docs/QUICKSTART.md) and `packages/node/sdk/README.md` for handler patterns and usage.
