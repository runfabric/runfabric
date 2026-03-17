# Provider Setup

Provider-by-provider checklist for credentials and deploy wiring. The Go engine uses REST/SDK for deploy where implemented (see [DEPLOY_PROVIDERS](DEPLOY_PROVIDERS.md)).

## Provider onboarding (quick steps)

1. Set **provider** in `runfabric.yml` (e.g. `provider.name: aws-lambda`).
2. Install provider adapter if your project uses npm packages (e.g. `@runfabric/provider-aws-lambda`); otherwise use the built-in engine.
3. Add provider-specific **extensions** in config when needed.
4. Export required **credential env vars** (see sections below).
5. Run **`runfabric doctor`**.
6. Run **`runfabric deploy`**.

Real deploy is opt-in: **`RUNFABRIC_REAL_DEPLOY=1`** or **`RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1`**. Simulated mode is default.

## Common flow

For any provider:

```bash
runfabric doctor -c runfabric.yml
runfabric plan -c runfabric.yml
runfabric build -c runfabric.yml
runfabric deploy -c runfabric.yml
```

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

**API-based deploy** (when `providerOverrides` uses `gcp-functions`):

- `GCP_ACCESS_TOKEN` — e.g. from `gcloud auth print-access-token` or a service account
- `GCP_PROJECT` or `GCP_PROJECT_ID`
- Source: either set `GCP_SOURCE_BUCKET` and `GCP_SOURCE_OBJECT` (pre-uploaded zip), or set `GCP_UPLOAD_BUCKET` to zip project root and upload before deploy. `runfabric logs` uses Cloud Logging (same token).

**CLI-based (optional):**

- `GCP_PROJECT_ID`, `GCP_SERVICE_ACCOUNT_KEY`
- `RUNFABRIC_GCP_REAL_DEPLOY=1` — built-in deploy/remove uses `gcloud`; overrides: `RUNFABRIC_GCP_DEPLOY_CMD`, `RUNFABRIC_GCP_DESTROY_CMD`

## Azure Functions

**API-based deploy** (default when provider is `azure-functions`):

- `AZURE_ACCESS_TOKEN` — e.g. from `az account get-access-token --resource https://management.azure.com`
- `AZURE_SUBSCRIPTION_ID`
- `AZURE_RESOURCE_GROUP` (optional; defaults to `service-stage`)
- Deploy creates resource group and function app via Management REST API; remove/invoke use API; logs return portal link. For CLI log fetch, set `AZURE_LOG_ANALYTICS_WORKSPACE_ID` (and `AZURE_ACCESS_TOKEN`) to query Log Analytics.

Optional CLI-based (legacy):

- `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID`, `AZURE_RESOURCE_GROUP`
- `RUNFABRIC_AZURE_REAL_DEPLOY=1` — built-in deploy/remove uses `func` + `az`; overrides: `RUNFABRIC_AZURE_DEPLOY_CMD`, `RUNFABRIC_AZURE_DESTROY_CMD`

## Kubernetes

**API-based deploy** (default when provider is `kubernetes`):

- `KUBECONFIG` (or in-cluster config when running inside the cluster)
- Deploy creates namespace, Deployment, and Service via client-go; remove deletes namespace; invoke uses URL from receipt (e.g. port-forward or ingress); logs fetch pod logs via client-go.

## Cloudflare Workers

**API-based deploy** (default when provider is `cloudflare-workers`):

- `CLOUDFLARE_ACCOUNT_ID`
- `CLOUDFLARE_API_TOKEN`
- Deploy uploads Worker script via API; remove/invoke/logs use Cloudflare API (tail for logs).

## Vercel

**API-based deploy** (default when provider is `vercel`):

- `VERCEL_TOKEN`
- `VERCEL_TEAM_ID` (optional, for teams)
- Deploy/remove/invoke/logs via Vercel API (deployments, project delete, HTTP invoke, deployment events).

## Netlify

**API-based deploy** (default when provider is `netlify`):

- `NETLIFY_AUTH_TOKEN`
- `NETLIFY_SITE_ID` (optional; site is created on first deploy if unset)
- Deploy creates site if needed and uploads zip; remove/invoke/logs via Netlify API.

## Alibaba FC

**API-based deploy** (default when provider is `alibaba-fc`):

- `ALIBABA_ACCESS_KEY_ID`, `ALIBABA_ACCESS_KEY_SECRET`
- `ALIBABA_FC_ACCOUNT_ID` (Alibaba Cloud account ID)
- `ALIBABA_FC_REGION` or `provider.region` (default `cn-hangzhou`)
- Deploy/remove/invoke via FC OpenAPI; logs return console link.

## DigitalOcean App Platform

**API-based deploy** (default when provider is `digitalocean-functions`):

- `DIGITALOCEAN_ACCESS_TOKEN`
- `DO_APP_REPO` (e.g. `owner/repo` for GitHub)
- `DO_REGION` (optional; default `ams`)
- Deploy/remove/invoke/logs via DigitalOcean Apps API.

## Fly Machines

**API-based deploy** (default when provider is `fly-machines`):

- `FLY_API_TOKEN`
- `FLY_ORG_ID` (optional; default `personal`)
- Deploy creates app via Fly Machines API; remove/invoke/logs use API.

## IBM OpenWhisk

**API-based deploy** (default when provider is `ibm-openwhisk`):

- `IBM_OPENWHISK_AUTH` (e.g. user:password or API key)
- `IBM_OPENWHISK_API_HOST` (optional; default `https://us-south.functions.cloud.ibm.com`)
- `IBM_OPENWHISK_NAMESPACE` (optional; default `_`)
- Deploy/remove/invoke/logs via OpenWhisk REST API.

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

- `examples/node/hello-http/runfabric.<provider>.yml`
- `examples/node/hello-http/runfabric.quickstart.yml`

Compose contracts example:

- `examples/node/compose-contracts/runfabric.compose.yml`
