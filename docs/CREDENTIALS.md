# Provider Credentials

`runfabric` reads credentials from environment variables (`process.env`).

For a minimal first run, use `examples/hello-http/runfabric.quickstart.yml` (requires only Cloudflare credentials).
For provider-specific configs, use files listed in `examples/hello-http/PROVIDERS.md`.

## Required Credentials By Provider

| Provider | Required Environment Variables |
| --- | --- |
| `aws-lambda` | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` |
| `gcp-functions` | `GCP_PROJECT_ID`, `GCP_SERVICE_ACCOUNT_KEY` |
| `azure-functions` | `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID`, `AZURE_RESOURCE_GROUP` |
| `cloudflare-workers` | `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` |
| `vercel` | `VERCEL_TOKEN`, `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID` |
| `netlify` | `NETLIFY_AUTH_TOKEN`, `NETLIFY_SITE_ID` |
| `alibaba-fc` | `ALICLOUD_ACCESS_KEY_ID`, `ALICLOUD_ACCESS_KEY_SECRET`, `ALICLOUD_REGION` |
| `digitalocean-functions` | `DIGITALOCEAN_ACCESS_TOKEN`, `DIGITALOCEAN_NAMESPACE` |
| `fly-machines` | `FLY_API_TOKEN`, `FLY_APP_NAME` |
| `ibm-openwhisk` | `IBM_CLOUD_API_KEY`, `IBM_CLOUD_REGION`, `IBM_CLOUD_NAMESPACE` |

## Deployment Mode Flags

| Flag | Description |
| --- | --- |
| `RUNFABRIC_STAGE` | Optional default stage to use when `--stage` is not provided on CLI commands. |
| `RUNFABRIC_CLOUDFLARE_REAL_DEPLOY` | Set to `1`/`true` to run real Cloudflare API deployment. If unset, Cloudflare deploy runs in simulated mode and writes a local receipt. |

## How To Pass Credentials

### Option 1: Export In Shell (recommended for local runs)
```bash
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_REGION="us-east-1"

runfabric doctor
runfabric deploy
```

### Option 2: Use `.env` File
```bash
cp .env.example .env
# edit .env values

set -a
source .env
set +a

runfabric doctor
runfabric deploy
```

### Option 3: One-off Command Prefix
```bash
AWS_ACCESS_KEY_ID="..." \
AWS_SECRET_ACCESS_KEY="..." \
AWS_REGION="us-east-1" \
runfabric deploy
```

## Optional CI Secret Mapping
If you add a deploy workflow, map the same variable names as GitHub repository secrets and expose them in workflow `env`.

Example:
```yaml
env:
  AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
  AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
  AWS_REGION: ${{ secrets.AWS_REGION }}
```

## Verification
- Run `runfabric doctor` to confirm required credentials for providers in `runfabric.yml`.
- Only credentials for providers listed in your `providers` block are required.
