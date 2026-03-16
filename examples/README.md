# RunFabric examples

Examples are grouped by runtime:

| Directory   | Description |
|------------|-------------|
| **node/**  | Node/JS/TS examples (hello-aws, hello-http, compose-app, compose-contracts, handler-scenarios) |
| **python/** | Python examples |
| **go/**    | Go examples |
| **java/**  | Java examples |
| **dotnet/** | .NET examples |

## Creating examples for all languages

To generate new examples with `runfabric init` for **js**, **ts**, **python**, and **go** (and any provider/template/state combination), see **[docs/EXAMPLES_ANALYSIS.md](../docs/EXAMPLES_ANALYSIS.md)**. That doc:

- Summarizes the [runfabric-example](https://github.com/runfabric/runfabric-example) repo (init commands, provider × template matrix).
- Gives the exact `runfabric init` pattern and examples for each language.
- Uses naming like `runfabric-<provider>-<trigger>-state-<backend>[-<lang>]`.

See also [docs/QUICKSTART.md](../docs/QUICKSTART.md) and [docs/SDK_FRAMEWORKS.md](../docs/SDK_FRAMEWORKS.md) for usage.
