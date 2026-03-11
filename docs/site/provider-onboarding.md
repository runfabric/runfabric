# Provider Onboarding

Each provider follows this sequence:

1. Add provider name to `providers` in `runfabric.yml`.
2. Add provider-specific `extensions` when needed.
3. Export required credential env vars.
4. Run `runfabric doctor`.
5. Run `runfabric deploy`.

## Real Deploy Mode

`runfabric` supports two deploy paths:

- simulated mode (default): deterministic endpoint + local receipt
- real mode (opt-in): provider CLI/API command output parsing

Global and per-provider flags:

- `RUNFABRIC_REAL_DEPLOY=1` to enable real mode globally
- `RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1` to enable per provider

Per-provider real deploy command env (for command-driven mode, must return JSON):

- `RUNFABRIC_AWS_DEPLOY_CMD` (optional override; AWS has built-in internal deployer when `RUNFABRIC_AWS_REAL_DEPLOY=1`)
- `RUNFABRIC_GCP_DEPLOY_CMD`
- `RUNFABRIC_AZURE_DEPLOY_CMD`
- `RUNFABRIC_CLOUDFLARE_REAL_DEPLOY=1` (direct API mode)
- `RUNFABRIC_VERCEL_DEPLOY_CMD`
- `RUNFABRIC_NETLIFY_DEPLOY_CMD`
- `RUNFABRIC_ALIBABA_DEPLOY_CMD`
- `RUNFABRIC_DIGITALOCEAN_DEPLOY_CMD`
- `RUNFABRIC_FLY_DEPLOY_CMD`
- `RUNFABRIC_IBM_DEPLOY_CMD`

Destroy command envs for remove/rollback:

- `RUNFABRIC_<PROVIDER>_DESTROY_CMD`

Provider-native observability command envs (optional):

- `RUNFABRIC_<PROVIDER>_TRACES_CMD`
- `RUNFABRIC_<PROVIDER>_METRICS_CMD`
