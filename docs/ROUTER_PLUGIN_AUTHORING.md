# Router Plugin Authoring Guide

This guide explains how to build a `kind=router` plugin for `runfabric router ...` flows.

## Contract Summary

Router plugins implement one RPC method:

- `Sync` — reconcile desired routing intent (`routing`, `zoneID`, `accountID`, `dryRun`) and return action records.

The engine invokes plugins over line-delimited JSON RPC on `stdin/stdout`.

Required RPC methods:

- `Handshake` -> returns `protocolVersion: "1"` and capabilities.
- `Sync` -> returns:
  - `dryRun` (`bool`)
  - `actions` (`[]`) where each action has `resource`, `action`, `name`, and optional `detail`.

## Quick Scaffold (Go)

Use the bundled template:

```bash
mkdir -p my-router-plugin
cp -R examples/router-plugin-go-template/* my-router-plugin/
cd my-router-plugin
go mod init github.com/your-org/my-router-plugin
go mod tidy
go build -o bin/my-router-plugin .
```

## Manifest Registration

Create `plugin.yaml` for external discovery (example):

```yaml
apiVersion: runfabric.io/plugin/v1
kind: router
id: my-router
name: My Router
version: 0.1.0
executable: ./bin/my-router-plugin
```

Then install/discover through your standard RunFabric plugin directory flow.

## Select Plugin

Configure `runfabric.yml`:

```yaml
extensions:
  routerPlugin: my-router
```

Optional router policy controls remain under `extensions.router` (auto apply, approvals, credentials, mutation policy, canary, quality scoring).

## Testing Checklist

1. `runfabric router routing --json` to confirm intent shape.
2. `runfabric router simulate --json` to verify local steering behavior.
3. `runfabric router dns-sync --dry-run --json` with your plugin selected.
4. `runfabric router chaos-verify --json` to validate failover scenarios.

## Built-in Expansion Adapters

Built-in routers now include:

- `cloudflare` (full DNS/LB reconciliation)
- `route53` (API reconciler)
- `ns1` (API reconciler)
- `azure-traffic-manager` (API reconciler)

Each reconciler computes idempotent create/update/no-op actions from routing intent and applies provider API changes when not running in dry-run mode.
