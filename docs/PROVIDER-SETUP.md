# Provider Setup

This document is a provider-by-provider checklist for credentials and deploy command wiring.

## Common Flow

For any provider config:

```bash
pnpm add -D @runfabric/provider-<provider-id>
runfabric doctor -c <provider-config.yml>
runfabric plan -c <provider-config.yml>
runfabric build -c <provider-config.yml>
runfabric deploy -c <provider-config.yml>
```

Install only the provider adapters used by your project.

Real mode is optional. Simulated mode is default.

## AWS Lambda

Required env:

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_REGION`

Optional real mode:

- `RUNFABRIC_AWS_REAL_DEPLOY=1`
- `RUNFABRIC_AWS_LAMBDA_ROLE_ARN` (required for built-in internal deployer)
- optional command overrides:
  - `RUNFABRIC_AWS_DEPLOY_CMD` (JSON output)
  - `RUNFABRIC_AWS_DESTROY_CMD`

## GCP Functions

Required env:

- `GCP_PROJECT_ID`
- `GCP_SERVICE_ACCOUNT_KEY`

Optional real mode:

- `RUNFABRIC_GCP_REAL_DEPLOY=1`
- built-in deploy/remove uses `gcloud` command contract
- optional overrides:
  - `RUNFABRIC_GCP_DEPLOY_CMD`
  - `RUNFABRIC_GCP_DESTROY_CMD`

## Azure Functions

Required env:

- `AZURE_TENANT_ID`
- `AZURE_CLIENT_ID`
- `AZURE_CLIENT_SECRET`
- `AZURE_SUBSCRIPTION_ID`
- `AZURE_RESOURCE_GROUP`

Optional real mode:

- `RUNFABRIC_AZURE_REAL_DEPLOY=1`
- built-in deploy/remove uses `func` + `az` command contract
- optional overrides:
  - `RUNFABRIC_AZURE_DEPLOY_CMD`
  - `RUNFABRIC_AZURE_DESTROY_CMD`

## Cloudflare Workers

Required env:

- `CLOUDFLARE_API_TOKEN`
- `CLOUDFLARE_ACCOUNT_ID`

Optional real mode:

- `RUNFABRIC_CLOUDFLARE_REAL_DEPLOY=1` (direct API path)
- optional destroy override: `RUNFABRIC_CLOUDFLARE_DESTROY_CMD` (otherwise built-in API destroy is used)

## Vercel

Required env:

- `VERCEL_TOKEN`
- `VERCEL_ORG_ID`
- `VERCEL_PROJECT_ID`

Optional real mode:

- `RUNFABRIC_VERCEL_REAL_DEPLOY=1`
- built-in deploy/remove uses `vercel` command contract
- optional overrides:
  - `RUNFABRIC_VERCEL_DEPLOY_CMD`
  - `RUNFABRIC_VERCEL_DESTROY_CMD`

## Netlify

Required env:

- `NETLIFY_AUTH_TOKEN`
- `NETLIFY_SITE_ID`

Optional real mode:

- `RUNFABRIC_NETLIFY_REAL_DEPLOY=1`
- built-in deploy/remove uses `netlify` command contract
- optional overrides:
  - `RUNFABRIC_NETLIFY_DEPLOY_CMD`
  - `RUNFABRIC_NETLIFY_DESTROY_CMD`

## Alibaba FC

Required env:

- `ALICLOUD_ACCESS_KEY_ID`
- `ALICLOUD_ACCESS_KEY_SECRET`
- `ALICLOUD_REGION`

Optional real mode:

- `RUNFABRIC_ALIBABA_REAL_DEPLOY=1`
- built-in deploy/remove uses `s` command contract
- optional overrides:
  - `RUNFABRIC_ALIBABA_DEPLOY_CMD`
  - `RUNFABRIC_ALIBABA_DESTROY_CMD`

## DigitalOcean Functions

Required env:

- `DIGITALOCEAN_ACCESS_TOKEN`
- `DIGITALOCEAN_NAMESPACE`

Optional real mode:

- `RUNFABRIC_DIGITALOCEAN_REAL_DEPLOY=1`
- built-in deploy/remove uses `doctl` command contract
- optional overrides:
  - `RUNFABRIC_DIGITALOCEAN_DEPLOY_CMD`
  - `RUNFABRIC_DIGITALOCEAN_DESTROY_CMD`

## Fly Machines

Required env:

- `FLY_API_TOKEN`
- `FLY_APP_NAME`

Optional real mode:

- `RUNFABRIC_FLY_REAL_DEPLOY=1`
- built-in deploy/remove uses `flyctl` command contract
- optional overrides:
  - `RUNFABRIC_FLY_DEPLOY_CMD`
  - `RUNFABRIC_FLY_DESTROY_CMD`

## IBM OpenWhisk

Required env:

- `IBM_CLOUD_API_KEY`
- `IBM_CLOUD_REGION`
- `IBM_CLOUD_NAMESPACE`

Optional real mode:

- `RUNFABRIC_IBM_REAL_DEPLOY=1`
- built-in deploy/remove uses `ibmcloud` command contract
- optional overrides:
  - `RUNFABRIC_IBM_DEPLOY_CMD`
  - `RUNFABRIC_IBM_DESTROY_CMD`

## Optional Provider-Native Traces/Metrics Commands

For providers where you want cloud-native observability calls instead of local artifact-derived data, set:

- `RUNFABRIC_<PROVIDER>_TRACES_CMD`
- `RUNFABRIC_<PROVIDER>_METRICS_CMD`

Example (AWS):

```bash
export RUNFABRIC_AWS_TRACES_CMD='echo "{\"traces\":[{\"timestamp\":\"2026-01-01T00:00:00Z\",\"message\":\"trace\"}]}"'
export RUNFABRIC_AWS_METRICS_CMD='echo "{\"metrics\":[{\"name\":\"invocations\",\"value\":10,\"unit\":\"count\"}]}"'
```

## Examples

Provider config examples:

- `examples/hello-http/runfabric.<provider>.yml`
- `examples/hello-http/runfabric.quickstart.yml`

Compose contracts example:

- `examples/compose-contracts/runfabric.compose.yml`
