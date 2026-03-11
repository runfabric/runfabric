# Command Reference

## Core

- `runfabric init [--template <api|worker|queue|cron>] [--provider <name>] [--lang <ts|js>] [--service <name>] [--pm <npm|pnpm|yarn|bun>] [--skip-install] [--call-local] [--no-interactive]`
- `runfabric doctor -c <config> [--stage <name>]`
- `runfabric plan -c <config> [--stage <name>] [--json]`
- `runfabric build -c <config> [--stage <name>] [--out <dir>] [--json]`
- `runfabric deploy -c <config> [--stage <name>] [--out <dir>] [--json]`
- `runfabric deploy function <name> -c <config> [--json]`
- `runfabric remove -c <config> [--stage <name>] [--provider <name>] [--json]`
- `runfabric call-local -c <config> [--serve] [--watch] [--host <host>] [--port <number>] [--provider <name>] [--method <GET|POST|...>] [--path </route>] [--query <k=v&k2=v2>] [--header <k:v>] [--body <text>] [--event <file>] [--entry <path>]`
- `runfabric invoke --provider <name> [--payload <json>]`
- `runfabric logs --provider <name>`
- `runfabric providers`
- `runfabric primitives`

## Compose

- `runfabric compose plan -f runfabric.compose.yml [--stage <name>] [--json]`
- `runfabric compose deploy -f runfabric.compose.yml [--stage <name>] [--json]`

## Failure and Recovery

- Deploy exit codes:
  - `0` all success
  - `2` partial failures
  - `1` full failure
- Optional deploy rollback: `RUNFABRIC_ROLLBACK_ON_FAILURE=1`
- Remove recovery notes: `.runfabric/recovery/remove/*.json`
