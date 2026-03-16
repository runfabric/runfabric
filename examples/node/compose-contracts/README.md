# Compose Contracts Example

This reference example demonstrates multi-service orchestration and endpoint contract naming.

## Services

- `api` (`cloudflare-workers`): exposes `GET /api`
- `worker` (`aws-lambda`): queue consumer depending on `api`
- `scheduler` (`gcp-functions`): cron trigger depending on `worker`

## Compose File

Use `examples/compose-contracts/runfabric.compose.yml`:

```bash
pnpm run runfabric -- compose plan -f examples/compose-contracts/runfabric.compose.yml
pnpm run runfabric -- compose deploy -f examples/compose-contracts/runfabric.compose.yml
```

## Cross-service Output Contract

`runfabric compose deploy` exports deployment endpoints as environment variables:

- `RUNFABRIC_OUTPUT_API_CLOUDFLARE_WORKERS_ENDPOINT`
- `RUNFABRIC_OUTPUT_WORKER_AWS_LAMBDA_ENDPOINT`
- `RUNFABRIC_OUTPUT_SCHEDULER_GCP_FUNCTIONS_ENDPOINT`

Naming convention:

- `RUNFABRIC_OUTPUT_<SERVICE_NAME_UPPER>_<PROVIDER_UPPER>_ENDPOINT`
- non-alphanumeric characters are normalized to `_`

Use these outputs in hooks, release scripts, smoke tests, or integration checks.
