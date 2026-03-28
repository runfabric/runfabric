# linode provider plugin

A Linode provider plugin implementing the RunFabric SDK provider contract for command-driven serverless on Linode.

This is an external plugin built purely against the plugin-sdk with no dependencies on the platform implementation. It serves as a reference implementation for SDK-only external providers.

## Build

```bash
go mod tidy
go build -o bin/linode-plugin .
```

## Install (local)

Copy this folder to:

```text
$RUNFABRIC_HOME/plugins/providers/linode/0.1.0/
```

Make sure `plugin.yaml` executable points to `./bin/linode-plugin`.

## Implementation

The complete Linode provider implementation is in `main.go` using only SDK contracts from `github.com/runfabric/runfabric/plugin-sdk/go`. This demonstrates the pattern for external providers: **never import from platform/, only from the SDK layer**.

## Contract

This scaffold implements methods expected by the external provider adapter:

- SDK-managed `Handshake` metadata via `server.HandshakeMetadata`
- `Doctor`
- `Plan`
- `Deploy`
- `Remove`
- `Invoke`
- `Logs`

## Credentials

- `LINODE_TOKEN` is used for `Doctor` and is forwarded to command-based operations.
- You can override the token env name with `tokenEnv` in config, or provide `token` directly in config if needed.

## Runtime Behavior

- `ValidateConfig` validates service shape and enforces `nodejs` / `python` runtimes.
- `Doctor` calls the Linode profile API at `https://api.linode.com/v4/profile` to verify credentials.
- `Plan` inspects the RunFabric config and reports the functions that would be deployed.
- `Invoke` can call a function URL directly when `invokeUrl` or `functions[].url` is configured.
- Artifact resolution order is `functions[].artifact`, `functions[].outputPath`, `.runfabric/<name>.zip`, `dist/<name>.zip`, then `build/<name>.zip`.
- Command-based operations require explicit command wiring (`config.commands.*`, `<operation>Command`, or `LINODE_*_CMD` env vars).
- Command-based `Invoke` still requires an explicit command when no function URL is configured.
- `Deploy`, `Remove`, `Invoke`, and `Logs` can execute external commands so the plugin stays decoupled from engine internals and provider-specific CLIs.

## Command Hooks

Configure one or more of these environment variables, or set the equivalent config keys:

- `LINODE_DEPLOY_CMD` or `commands.deploy`
- `LINODE_REMOVE_CMD` or `commands.remove`
- `LINODE_INVOKE_CMD` or `commands.invoke`
- `LINODE_LOGS_CMD` or `commands.logs`

Each command runs through `/bin/sh -lc` and receives these environment variables:

- `RUNFABRIC_PROVIDER`
- `RUNFABRIC_SERVICE`
- `RUNFABRIC_STAGE`
- `RUNFABRIC_ROOT`
- `RUNFABRIC_FUNCTION`
- `RUNFABRIC_PAYLOAD_BASE64`
- `RUNFABRIC_RUNTIME`
- `RUNFABRIC_ENTRY`
- `RUNFABRIC_ARTIFACT_PATH`
- `RUNFABRIC_ARTIFACT_DIR`
- `RUNFABRIC_ARTIFACT_BASENAME`
- `LINODE_TOKEN`
- `RUNFABRIC_LINODE_APP_ID` (when `appID` is configured)

If a command prints JSON matching the SDK result shape, the plugin returns that structured response. Otherwise stdout is returned as plain output.

## Example

```bash
export LINODE_TOKEN=...
export LINODE_DEPLOY_CMD='linode-cli functions action-create "$RUNFABRIC_SERVICE-$RUNFABRIC_STAGE-$RUNFABRIC_FUNCTION"'
export LINODE_REMOVE_CMD='linode-cli functions action-delete "$RUNFABRIC_SERVICE-$RUNFABRIC_STAGE-$RUNFABRIC_FUNCTION"'
export LINODE_LOGS_CMD='linode-cli functions activation-list "$RUNFABRIC_SERVICE-$RUNFABRIC_STAGE-$RUNFABRIC_FUNCTION"'
```
