# RunFabric MCP Server

The RunFabric **MCP (Model Context Protocol)** server exposes plan, deploy, doctor, remove, and invoke as tools for AI agents and IDEs (e.g. Cursor, Claude Code).

## Scope

- **Tools:** `runfabric_doctor`, `runfabric_plan`, `runfabric_deploy`, `runfabric_remove`, `runfabric_invoke`
- **Transport:** stdio (process-spawned). The client launches the server and communicates via stdin/stdout.
- **Requirement:** The `runfabric` CLI must be on `PATH`. Build from repo with `make build`; the binary is at `bin/runfabric`. Override with `RUNFABRIC_CMD` if the binary has another name or path.

## Running the server

From the repo root:

```bash
cd protocol/mcp && npm install && npm run build && node dist/index.js
```

Or after build, run the binary:

```bash
node protocol/mcp/dist/index.js
```

Ensure `runfabric` is on `PATH` (e.g. `export PATH="$PWD/bin:$PATH"` from repo root after `make build`).

## Adding to Cursor

In Cursor settings (or `.cursor/mcp.json`), add an MCP server that runs the RunFabric MCP process:

- **Command:** `node`
- **Args:** `path/to/unifn-framework-scaffold/protocol/mcp/dist/index.js`
- **Env (optional):** `RUNFABRIC_CMD=/path/to/runfabric` if the CLI is not on PATH

The exact configuration format depends on your Cursor/MCP client; use the stdio transport and point to the built `dist/index.js` (or `npm start` from `protocol/mcp`).

## Tool parameters

| Tool | Parameters |
|------|------------|
| `runfabric_doctor` | `configPath` (optional), `stage` (optional) |
| `runfabric_plan` | `configPath`, `stage`, `provider` (optional) |
| `runfabric_deploy` | `configPath`, `stage`, `preview`, `provider` (optional) |
| `runfabric_remove` | `configPath`, `stage`, `provider` (optional) |
| `runfabric_invoke` | `configPath`, `stage`, `function` (required), `payload` (optional), `provider` (optional) |

All tools run the CLI with `--json` and return the combined stdout/stderr as text content. When the exit code is non-zero, the result is marked with `isError: true`.

## See also

- [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md) — CLI commands and flags.
- [Model Context Protocol](https://modelcontextprotocol.io/) — MCP specification.
