# Command Reference

## Core

- `runfabric init [--template <api|worker|queue|cron>] [--provider <name>] [--lang <ts|js>] [--service <name>] [--pm <npm|pnpm|yarn|bun>] [--skip-install] [--call-local] [--no-interactive]`
- `runfabric doctor -c <config> [--stage <name>]`
- `runfabric plan -c <config> [--stage <name>] [--json]`
- `runfabric build -c <config> [--stage <name>] [--out <dir>] [--json]`
- `runfabric package -c <config> [--stage <name>] [--function <name>] [--out <dir>] [--json]`
- `runfabric deploy -c <config> [--stage <name>] [--function <name>] [--out <dir>] [--json]`
- `runfabric deploy fn <name> -c <config> [--stage <name>] [--out <dir>] [--json]`
- `runfabric deploy function <name> -c <config> [--stage <name>] [--out <dir>] [--json]`
- `runfabric deploy-function <name> -c <config> [--stage <name>] [--out <dir>] [--json]`
- `runfabric remove -c <config> [--stage <name>] [--provider <name>] [--json]`
- `runfabric migrate --input <serverless.yml> [--output <runfabric.yml>] [--provider <id>] [--dry-run] [--force] [--json]`
- `runfabric call-local -c <config> [--serve] [--watch] [--host <host>] [--port <number>] [--provider <name>] [--method <GET|POST|...>] [--path </route>] [--query <k=v&k2=v2>] [--header <k:v>] [--body <text>] [--event <file>] [--entry <path>]`
- `runfabric dev -c <config> [--preset <http|queue|storage>] [--watch|--no-watch] [--once] [--host <host>] [--port <number>] [--method <GET|POST|...>] [--path </route>] [--query <k=v>] [--header <k:v>] [--interval-seconds <n>]`
- `runfabric invoke --provider <name> [--payload <json>]`
- `runfabric logs --provider <name>`
- `runfabric traces [--config <path>] --provider <name> [--since <iso>] [--correlation-id <id>] [--limit <count>] [--json]`
- `runfabric metrics [--config <path>] --provider <name> [--since <iso>] [--json]`
- `runfabric providers`
- `runfabric primitives`

`invoke` and `logs` resolve project context from the current working directory.

## Compose

- `runfabric compose plan -f runfabric.compose.yml [--stage <name>] [--json]`
- `runfabric compose deploy -f runfabric.compose.yml [--stage <name>] [--json]`

## State

- `runfabric state pull --provider <name> [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--service <name>] [--json]`
- `runfabric state list [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--service <name>] [--provider <name>] [--json]`
- `runfabric state backup [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--service <name>] [--provider <name>] [--out <path>] [--json]`
- `runfabric state restore --file <path> [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--json]`
- `runfabric state force-unlock --provider <name> [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--service <name>] [--json]`
- `runfabric state migrate --from <backend> --to <backend> [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--service <name>] [--provider <name>] [--json]`
- `runfabric state reconcile [--config <path>] [--backend <local|postgres|s3|gcs|azblob>] [--stage <name>] [--service <name>] [--provider <name>] [--json]`

## Failure and Recovery

- Deploy exit codes:
  - `0` all success
  - `2` partial failures
  - `1` full failure
- Optional deploy rollback: `RUNFABRIC_ROLLBACK_ON_FAILURE=1`
- Remove recovery notes: `.runfabric/recovery/remove/*.json`
