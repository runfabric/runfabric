# Provider Onboarding

Each provider follows this sequence:

1. Add provider name to `providers` in `runfabric.yml`.
2. Install provider adapter package (`@runfabric/provider-<provider-id>`).
3. Add provider-specific `extensions` when needed.
4. Export required credential env vars.
5. Run `runfabric doctor`.
6. Run `runfabric deploy`.

## Real Deploy Mode

`runfabric` supports two deploy paths:

- simulated mode (default): deterministic endpoint + local receipt
- real mode (opt-in): provider CLI/API command output parsing

Global and per-provider flags:

- `RUNFABRIC_REAL_DEPLOY=1` to enable real mode globally
- `RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1` to enable per provider

Built-in real deployers are used by default in real mode:

- `aws-lambda`: AWS SDK path
- `cloudflare-workers`: direct Cloudflare API path
- `gcp-functions|azure-functions|vercel|netlify|alibaba-fc|digitalocean-functions|fly-machines|ibm-openwhisk`:
  built-in provider CLI command contracts

Optional override envs (deploy command should return JSON on stdout):

- `RUNFABRIC_<PROVIDER>_DEPLOY_CMD`
- `RUNFABRIC_<PROVIDER>_DESTROY_CMD`

Provider-native observability command envs (optional):

- `RUNFABRIC_<PROVIDER>_TRACES_CMD`
- `RUNFABRIC_<PROVIDER>_METRICS_CMD`
