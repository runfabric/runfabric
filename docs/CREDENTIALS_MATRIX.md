# Credentials Matrix

Quick reference for credential wiring across providers and state backends.

Source of truth for detailed setup:

- Provider credentials and real deploy behavior: `docs/CREDENTIALS.md`
- State backend schema, permissions, and locking: `docs/STATE_BACKENDS.md`

## Provider Credential Matrix

| Provider | Required Credentials |
| --- | --- |
| `aws-lambda` | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` |
| `gcp-functions` | `GCP_PROJECT_ID`, `GCP_SERVICE_ACCOUNT_KEY` |
| `azure-functions` | `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID`, `AZURE_RESOURCE_GROUP` |
| `kubernetes` | `KUBECONFIG`, `KUBE_CONTEXT`, `KUBE_NAMESPACE` |
| `cloudflare-workers` | `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` |
| `vercel` | `VERCEL_TOKEN`, `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID` |
| `netlify` | `NETLIFY_AUTH_TOKEN`, `NETLIFY_SITE_ID` |
| `alibaba-fc` | `ALICLOUD_ACCESS_KEY_ID`, `ALICLOUD_ACCESS_KEY_SECRET`, `ALICLOUD_REGION` |
| `digitalocean-functions` | `DIGITALOCEAN_ACCESS_TOKEN`, `DIGITALOCEAN_NAMESPACE` |
| `fly-machines` | `FLY_API_TOKEN`, `FLY_APP_NAME` |
| `ibm-openwhisk` | `IBM_CLOUD_API_KEY`, `IBM_CLOUD_REGION`, `IBM_CLOUD_NAMESPACE` |

## State Backend Credential Matrix

| State Backend | Required Credentials |
| --- | --- |
| `local` | none |
| `postgres` | `RUNFABRIC_STATE_POSTGRES_URL` (or custom env named by `state.postgres.connectionStringEnv`) |
| `s3` | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` (or equivalent AWS credential chain) |
| `gcs` | `GOOGLE_APPLICATION_CREDENTIALS` (or workload identity) |
| `azblob` | `AZURE_STORAGE_CONNECTION_STRING` OR `AZURE_STORAGE_ACCOUNT` + `AZURE_STORAGE_KEY` |

## Common Deploy Controls

| Control | Purpose |
| --- | --- |
| `RUNFABRIC_STAGE` | default stage when `--stage` is not passed |
| `RUNFABRIC_REAL_DEPLOY=1` | enable real deploy globally |
| `RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1` | enable real deploy for one provider |
| `RUNFABRIC_ROLLBACK_ON_FAILURE=1` | legacy fallback toggle for rollback-on-failure behavior |
