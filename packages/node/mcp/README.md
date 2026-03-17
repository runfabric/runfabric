# @runfabric/mcp-stdio

MCP server that exposes RunFabric **doctor**, **plan**, **deploy**, **remove**, and **invoke** as tools for AI agents and IDEs. **Requires the RunFabric CLI binary (`runfabric`) on PATH or set `RUNFABRIC_CMD`.**

## Requirement: RunFabric CLI

This package does **not** bundle the RunFabric CLI. The `runfabric` binary must be available:

1. **From repo:** Run `make build` in the RunFabric repo, then put `bin/runfabric` on your PATH (e.g. `export PATH="/path/to/repo/bin:$PATH"`).
2. **Override path:** Set env `RUNFABRIC_CMD` to the full path of the binary (e.g. `RUNFABRIC_CMD=/path/to/repo/bin/runfabric`).

If the CLI is missing, MCP tools return an error that includes this remedy.

### For AI agents

- **Problem:** MCP tool fails with "RunFabric CLI not found" or spawn ENOENT.
- **Cause:** The `runfabric` executable is not on PATH and `RUNFABRIC_CMD` is not set.
- **Fix (pick one):**
  - Build: `make build` in the project root, then `export PATH="$(pwd)/bin:$PATH"`.
  - Or set: `RUNFABRIC_CMD=/absolute/path/to/runfabric` (env for the process that starts the MCP server).
- **Verify:** Run `runfabric --help` or `runfabric doctor -c runfabric.yml` in a shell; if that works, MCP tools will work once PATH or RUNFABRIC_CMD is set for the MCP process.

## Install

```bash
npm install @runfabric/mcp-stdio
```

## Usage

- **CLI:** After install, the `runfabric-mcp` command starts the MCP server (stdio). Use it in your IDE’s MCP config.
- **Cursor:** In MCP settings, set command to `runfabric-mcp` (or `npx runfabric-mcp`) and ensure `runfabric` is on `PATH` or `RUNFABRIC_CMD` is set.

## Tools

| Tool | Description |
|------|-------------|
| `runfabric_doctor` | Validate config and credentials |
| `runfabric_plan` | Show deployment plan |
| `runfabric_build` | Build artifacts for deploy |
| `runfabric_deploy` | Deploy |
| `runfabric_remove` | Remove deployed resources |
| `runfabric_invoke` | Invoke a deployed function (requires `function`; optional `payload`) |
| `runfabric_logs` | Fetch logs (optional `function`; set `all: true` for all functions) |
| `runfabric_list` | List functions and deployment status |
| `runfabric_inspect` | Show lock, journal, and receipt state |
| `runfabric_releases` | List deployment history (releases) for the stage |

All tools run the CLI with `--json`, `--non-interactive`, and `--yes`.

## See also

- [MCP.md](../../docs/MCP.md) in the repo for full setup and Cursor config.
