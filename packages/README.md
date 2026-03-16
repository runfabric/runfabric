# RunFabric packages

Per-runtime CLI and SDK packages (canonical location; the legacy `sdk/` directory has been removed). See [docs/FILE_STRUCTURE.md](../docs/FILE_STRUCTURE.md) for the full layout and naming.

| Runtime | CLI package      | SDK package       | Path              |
|---------|------------------|-------------------|-------------------|
| Node    | @runfabric/cli   | @runfabric/sdk    | node/cli, node/sdk |
| Python  | runfabric        | *(in same pkg)*   | python/runfabric  |
| Go      | *(engine binary)*| —                 | go/sdk            |
| Java    | —                | io.runfabric:runfabric-sdk | java/sdk |
| .NET    | —                | RunFabric.Sdk     | dotnet/sdk        |

- **Node:** `packages/node/cli` (CLI wrapper + programmatic API), `packages/node/sdk` (handler contract, HTTP adapter, Express/Fastify/Nest).
- **Python:** `packages/python/runfabric` (CLI + SDK in one package; future split into cli/ and sdk/).
- **Go / Java / .NET:** SDK-only under `packages/<runtime>/sdk`.
