# RunFabric packaging

Packaging strategy for CLI and language SDKs.

## Quick navigation

- **Where each runtime’s CLI/SDK lives**: Target layout
- **What exists today**: Current state

## Target layout

| Runtime    | CLI                                         | SDK package                                                                    |
| ---------- | ------------------------------------------- | ------------------------------------------------------------------------------ |
| **Node**   | `@runfabric/cli` (wrapper on engine binary) | `@runfabric/sdk` (handler contract + HTTP adapter + framework adapters)        |
| **Python** | `runfabric` (CLI, wrapper on engine binary) | `@runfabric/sdk-python` (handler contract + HTTP adapter + framework adapters) |
| **Go**     | `runfabric` (CLI, wrapper on engine binary) | `@runfabric/sdk-go` (handler contract + HTTP adapter + framework adapters)     |
| **Java**   | _(same Go/engine binary)_                   | `@runfabric/sdk-java` (handler contract + HTTP adapter + framework adapters)   |
| **.NET**   | `runfabric` (CLI, wrapper on engine binary) | `@runfabric/sdk-dotnet` (handler contract + HTTP adapter + framework adapters) |

- **CLI**: invokes or embeds the Go engine binary; same UX across runtimes.
- **SDK**: handler contract `(event, context) -> response`, HTTP request/response adapter, and optional framework adapters (Express, Fastify, FastAPI, Flask, etc.). See [SDK_FRAMEWORKS.md](SDK_FRAMEWORKS.md).

## Current state

- **Node**: **@runfabric/cli** (`packages/node/cli`) — CLI wrapper only; **@runfabric/sdk** (`packages/node/sdk`) — handler contract, HTTP adapter, framework adapters.
- **Python** (`packages/python/runfabric`): single package `runfabric` that bundles CLI and SDK. Future: split into `packages/python/cli` and `packages/python/sdk` (runfabric-sdk).
- **Go / Java / .NET** (`packages/go/sdk`, `packages/java/sdk`, `packages/dotnet/sdk`): SDK-only packages; CLI is the Go binary built from `cmd/runfabric`.
