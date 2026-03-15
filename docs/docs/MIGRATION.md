# Migration Guide

This guide helps teams move from scaffold-style deploys to command/API-backed real provider deploys.

## Current Model

`runfabric` supports two deploy modes:

- simulated mode (default)
- real mode (opt-in)

Simulated mode still records deployment receipts/state so you can validate workflow shape before touching cloud resources.

## Serverless Framework To runfabric Bootstrap

Use built-in migration tooling for a best-effort initial conversion:

```bash
runfabric migrate --input ./serverless.yml --output ./runfabric.yml --json
```

Migration notes:

- maps provider names (for example `aws` -> `aws-lambda`)
- maps `provider.runtime` into `runtime` (`node*` -> `nodejs`)
- maps function handlers into `entry` and `functions[].entry`
- maps common event types: `http|httpApi|schedule|sqs|s3`
- emits warnings when values need manual follow-up

## Migration Path

1. Keep existing `runfabric.yml` provider config.
2. Export required provider credentials.
3. Enable real mode:
   - `RUNFABRIC_REAL_DEPLOY=1` or `RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1`
4. Configure real deploy execution:
   - AWS: set `RUNFABRIC_AWS_LAMBDA_ROLE_ARN` (built-in AWS SDK deployer), and
   - for non-AWS providers ensure provider CLI/API prerequisites are installed/authenticated.
   - optional for any provider: set `RUNFABRIC_<PROVIDER>_DEPLOY_CMD` override returning JSON.
5. Run `runfabric doctor`, `plan`, `build`, `deploy`.
6. Validate endpoint and receipt output.
7. Configure cleanup:
   - built-in destroy executes per provider in real mode, or
   - set `RUNFABRIC_<PROVIDER>_DESTROY_CMD` override for custom cleanup/rollback workflows.

## Example (AWS)

```bash
export RUNFABRIC_AWS_REAL_DEPLOY=1
export RUNFABRIC_AWS_LAMBDA_ROLE_ARN='arn:aws:iam::123456789012:role/runfabric-lambda-role'

runfabric deploy -c runfabric.aws-lambda.yml
```

## Definition Of Done Per Provider

- `doctor` validates required credentials.
- `deploy` returns endpoint from provider response parsing.
- receipt/state files are written.
- `invoke` and `logs` are available.
- `remove` path uses provider destroy and local cleanup.
