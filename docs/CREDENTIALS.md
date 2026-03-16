# Provider Credentials

`runfabric` reads credentials from environment variables.

## How To Pass Credentials

### Option 1: Export in shell

```bash
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_REGION="us-east-1"

runfabric doctor
runfabric deploy
```

### Option 2: `.env` + source

```bash
cp .env.example .env
# edit values

set -a
source .env
set +a

runfabric doctor
runfabric deploy
```

### Option 3: One-off command prefix

```bash
AWS_ACCESS_KEY_ID="..." AWS_SECRET_ACCESS_KEY="..." AWS_REGION="us-east-1" runfabric deploy
```

## Deploy Mode Controls

- `RUNFABRIC_STAGE`: default stage when `--stage` is not provided.
- `RUNFABRIC_REAL_DEPLOY=1`: enable real mode globally for all providers.
- Rollback on failure precedence:
  - `runfabric deploy --rollback-on-failure|--no-rollback-on-failure`
  - `deploy.rollbackOnFailure` in `runfabric.yml`
  - legacy env fallback `RUNFABRIC_ROLLBACK_ON_FAILURE=1`

Per-provider real mode flag:

- `RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1`

(Examples: `RUNFABRIC_AWS_REAL_DEPLOY`, `RUNFABRIC_GCP_REAL_DEPLOY`, `RUNFABRIC_IBM_REAL_DEPLOY`)

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

Install the corresponding provider adapter package in your project (for example `@runfabric/provider-aws-lambda`).

## State Backend Credential Matrix

| State Backend | Required Credentials |
| --- | --- |
| `local` | none |
| `postgres` | `RUNFABRIC_STATE_POSTGRES_URL` (or custom env named by `state.postgres.connectionStringEnv`) |
| `s3` | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` (or equivalent AWS credential chain) |
| `gcs` | `GOOGLE_APPLICATION_CREDENTIALS` (or workload identity) |
| `azblob` | `AZURE_STORAGE_CONNECTION_STRING` OR `AZURE_STORAGE_ACCOUNT` + `AZURE_STORAGE_KEY` |

## Real Deploy Execution Matrix

When real mode is enabled, every provider has a built-in deployer path. Command envs are optional overrides.

| Provider | Built-in Real Deploy Path | Optional Override Env |
| --- | --- | --- |
| `aws-lambda` | AWS SDK deploy + destroy | `RUNFABRIC_AWS_DEPLOY_CMD`, `RUNFABRIC_AWS_DESTROY_CMD` |
| `gcp-functions` | built-in `gcloud` command contract | `RUNFABRIC_GCP_DEPLOY_CMD`, `RUNFABRIC_GCP_DESTROY_CMD` |
| `azure-functions` | built-in `func/az` command contract | `RUNFABRIC_AZURE_DEPLOY_CMD`, `RUNFABRIC_AZURE_DESTROY_CMD` |
| `kubernetes` | built-in `kubectl` command contract | `RUNFABRIC_KUBERNETES_DEPLOY_CMD`, `RUNFABRIC_KUBERNETES_DESTROY_CMD` |
| `cloudflare-workers` | Cloudflare Workers API deploy + destroy | `RUNFABRIC_CLOUDFLARE_DESTROY_CMD` |
| `vercel` | built-in `vercel` command contract | `RUNFABRIC_VERCEL_DEPLOY_CMD`, `RUNFABRIC_VERCEL_DESTROY_CMD` |
| `netlify` | built-in `netlify` command contract | `RUNFABRIC_NETLIFY_DEPLOY_CMD`, `RUNFABRIC_NETLIFY_DESTROY_CMD` |
| `alibaba-fc` | built-in `s` command contract | `RUNFABRIC_ALIBABA_DEPLOY_CMD`, `RUNFABRIC_ALIBABA_DESTROY_CMD` |
| `digitalocean-functions` | built-in `doctl` command contract | `RUNFABRIC_DIGITALOCEAN_DEPLOY_CMD`, `RUNFABRIC_DIGITALOCEAN_DESTROY_CMD` |
| `fly-machines` | built-in `flyctl` command contract | `RUNFABRIC_FLY_DEPLOY_CMD`, `RUNFABRIC_FLY_DESTROY_CMD` |
| `ibm-openwhisk` | built-in `ibmcloud` command contract | `RUNFABRIC_IBM_DEPLOY_CMD`, `RUNFABRIC_IBM_DESTROY_CMD` |

Notes:

- For command-contract providers, ensure the relevant provider CLI is installed and authenticated in your environment.
- Override commands should return JSON on stdout for deploy parsing.

## Provider Observability Command Matrix

Optional provider-native overrides for `runfabric traces` and `runfabric metrics`.
If these are unset, runfabric falls back to local artifact-derived traces/metrics.

| Provider | Traces Command Env | Metrics Command Env |
| --- | --- | --- |
| `aws-lambda` | `RUNFABRIC_AWS_TRACES_CMD` | `RUNFABRIC_AWS_METRICS_CMD` |
| `gcp-functions` | `RUNFABRIC_GCP_TRACES_CMD` | `RUNFABRIC_GCP_METRICS_CMD` |
| `azure-functions` | `RUNFABRIC_AZURE_TRACES_CMD` | `RUNFABRIC_AZURE_METRICS_CMD` |
| `kubernetes` | `RUNFABRIC_KUBERNETES_TRACES_CMD` | `RUNFABRIC_KUBERNETES_METRICS_CMD` |
| `cloudflare-workers` | `RUNFABRIC_CLOUDFLARE_TRACES_CMD` | `RUNFABRIC_CLOUDFLARE_METRICS_CMD` |
| `vercel` | `RUNFABRIC_VERCEL_TRACES_CMD` | `RUNFABRIC_VERCEL_METRICS_CMD` |
| `netlify` | `RUNFABRIC_NETLIFY_TRACES_CMD` | `RUNFABRIC_NETLIFY_METRICS_CMD` |
| `alibaba-fc` | `RUNFABRIC_ALIBABA_TRACES_CMD` | `RUNFABRIC_ALIBABA_METRICS_CMD` |
| `digitalocean-functions` | `RUNFABRIC_DIGITALOCEAN_TRACES_CMD` | `RUNFABRIC_DIGITALOCEAN_METRICS_CMD` |
| `fly-machines` | `RUNFABRIC_FLY_TRACES_CMD` | `RUNFABRIC_FLY_METRICS_CMD` |
| `ibm-openwhisk` | `RUNFABRIC_IBM_TRACES_CMD` | `RUNFABRIC_IBM_METRICS_CMD` |

Example output contract:

```json
{"traces":[{"timestamp":"2026-01-01T00:00:00.000Z","message":"trace line"}]}
```

```json
{"metrics":[{"name":"invocations","value":42,"unit":"count"}]}
```

## Examples

### AWS real deploy mode

```bash
export RUNFABRIC_AWS_REAL_DEPLOY=1
export RUNFABRIC_AWS_LAMBDA_ROLE_ARN='arn:aws:iam::123456789012:role/runfabric-lambda-role'

runfabric deploy -c runfabric.yml
```

If the execution role does not exist yet, create it once:

```bash
aws iam create-role \
  --role-name runfabric-lambda-exec \
  --assume-role-policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}'

aws iam attach-role-policy \
  --role-name runfabric-lambda-exec \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

export RUNFABRIC_AWS_LAMBDA_ROLE_ARN="$(aws iam get-role --role-name runfabric-lambda-exec --query 'Role.Arn' --output text)"
```

If AWS returns assume-role errors right after role creation, wait 20-60 seconds and retry deploy.

For real AWS deployments, set `RUNFABRIC_AWS_REAL_DEPLOY=1` for both `deploy` and `remove`. If omitted during remove, runfabric now fails fast instead of silently skipping cloud deletion.

Optional command overrides for custom AWS workflows:

```bash
export RUNFABRIC_AWS_DEPLOY_CMD='aws lambda create-function-url-config --function-name my-fn --output json'
export RUNFABRIC_AWS_DESTROY_CMD='aws lambda delete-function-url-config --function-name my-fn'
```

### Vercel real deploy mode

```bash
export RUNFABRIC_VERCEL_REAL_DEPLOY=1

runfabric deploy -c runfabric.yml
```

Optional override for custom Vercel workflow:

```bash
export RUNFABRIC_VERCEL_DEPLOY_CMD='vercel deploy --yes --prod --json'
```

## Validation

`runfabric doctor` checks that required provider credentials are set. It uses the same matrix as this document: for the configured provider (e.g. `aws-lambda`), it reports **provider-credentials** OK when all required env vars are non-empty, or lists missing/empty variables. Programmatic use: the engine’s `secrets` package exposes `RequiredProviderEnvVars(provider)` and `MissingProviderEnvVars(provider)` for validation or tooling.

## CI Secret Wiring

Map secret names to the same env variable names in workflow `env`.

Example:

```yaml
env:
  AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
  AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
  AWS_REGION: ${{ secrets.AWS_REGION }}
```

Then run:

```bash
pnpm run runfabric -- doctor -c <config>
pnpm run runfabric -- deploy -c <config>
```
