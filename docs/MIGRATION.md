# Migration Guide

This guide helps teams move from scaffold-style deploys to command/API-backed real provider deploys.

## Current Model

`runfabric` supports two deploy modes:

- simulated mode (default)
- real mode (opt-in)

Simulated mode still records deployment receipts/state so you can validate workflow shape before touching cloud resources.

## Migration Path

1. Keep existing `runfabric.yml` provider config.
2. Export required provider credentials.
3. Enable real mode:
   - `RUNFABRIC_REAL_DEPLOY=1` or `RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1`
4. Set provider deploy command env that returns JSON.
5. Run `runfabric doctor`, `plan`, `build`, `deploy`.
6. Validate endpoint and receipt output.
7. Add destroy command env for cleanup and rollback.

## Example (AWS)

```bash
export RUNFABRIC_AWS_REAL_DEPLOY=1
export RUNFABRIC_AWS_DEPLOY_CMD='aws lambda create-function-url-config --function-name my-fn --output json'
export RUNFABRIC_AWS_DESTROY_CMD='aws lambda delete-function-url-config --function-name my-fn'

runfabric deploy -c runfabric.aws-lambda.yml
```

## Definition Of Done Per Provider

- `doctor` validates required credentials.
- `deploy` returns endpoint from provider response parsing.
- receipt/state files are written.
- `invoke` and `logs` are available.
- `remove` path uses provider destroy and local cleanup.
