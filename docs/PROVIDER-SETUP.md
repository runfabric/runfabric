# Provider Setup Playbooks

This guide provides provider-by-provider bootstrap steps before running:

- `runfabric doctor`
- `runfabric deploy`

For the full credential variable matrix, see `docs/CREDENTIALS.md`.

## Common Workflow

1. Pick your provider config (`examples/hello-http/runfabric.<provider>.yml`).
2. Create/verify cloud account and target project/subscription/app.
3. Provision least-privilege credentials.
4. Export credentials in your shell or `.env`.
5. Run diagnostics:

```bash
runfabric doctor -c <path-to-config>
runfabric deploy -c <path-to-config>
```

## AWS Lambda

### Bootstrap

1. Create an AWS account/project boundary (account or sub-account strategy).
2. Create IAM user or role for deploy automation.
3. Grant function deployment permissions (Lambda + required networking/logging roles).

### Required Env

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_REGION`

### Verify

```bash
runfabric doctor -c examples/hello-http/runfabric.aws-lambda.yml
```

## Google Cloud Functions

### Bootstrap

1. Create/select GCP project.
2. Enable Cloud Functions and related APIs.
3. Create service account for deployment.
4. Grant Cloud Functions and supporting permissions.

### Required Env

- `GCP_PROJECT_ID`
- `GCP_SERVICE_ACCOUNT_KEY`

### Verify

```bash
runfabric doctor -c examples/hello-http/runfabric.gcp-functions.yml
```

## Azure Functions

### Bootstrap

1. Create/select subscription and resource group.
2. Create or identify Function App deployment target.
3. Create service principal with scoped permissions.

### Required Env

- `AZURE_TENANT_ID`
- `AZURE_CLIENT_ID`
- `AZURE_CLIENT_SECRET`
- `AZURE_SUBSCRIPTION_ID`
- `AZURE_RESOURCE_GROUP`

### Verify

```bash
runfabric doctor -c examples/hello-http/runfabric.azure-functions.yml
```

## Cloudflare Workers

### Bootstrap

1. Create API token with Workers deployment permissions.
2. Copy account ID.
3. Optional real deploy mode for API provisioning.

### Required Env

- `CLOUDFLARE_API_TOKEN`
- `CLOUDFLARE_ACCOUNT_ID`

### Optional Env

- `RUNFABRIC_CLOUDFLARE_REAL_DEPLOY=1` for real API deploy
- `RUNFABRIC_CLOUDFLARE_REAL_DEPLOY=0` for simulated receipt mode

### Verify

```bash
runfabric doctor -c examples/hello-http/runfabric.cloudflare-workers.yml
```

## Vercel

### Bootstrap

1. Create personal/team token.
2. Identify org/user ID.
3. Identify project ID.

### Required Env

- `VERCEL_TOKEN`
- `VERCEL_ORG_ID`
- `VERCEL_PROJECT_ID`

### Verify

```bash
runfabric doctor -c examples/hello-http/runfabric.vercel.yml
```

## Netlify

### Bootstrap

1. Create personal access token.
2. Identify site ID.

### Required Env

- `NETLIFY_AUTH_TOKEN`
- `NETLIFY_SITE_ID`

### Verify

```bash
runfabric doctor -c examples/hello-http/runfabric.netlify.yml
```

## Alibaba Function Compute

### Bootstrap

1. Create RAM access key pair for deployment automation.
2. Select target region and service scope.

### Required Env

- `ALICLOUD_ACCESS_KEY_ID`
- `ALICLOUD_ACCESS_KEY_SECRET`
- `ALICLOUD_REGION`

### Verify

```bash
runfabric doctor -c examples/hello-http/runfabric.alibaba-fc.yml
```

## DigitalOcean Functions

### Bootstrap

1. Create API token.
2. Create/identify namespace for functions.

### Required Env

- `DIGITALOCEAN_ACCESS_TOKEN`
- `DIGITALOCEAN_NAMESPACE`

### Verify

```bash
runfabric doctor -c examples/hello-http/runfabric.digitalocean-functions.yml
```

## Fly Machines

### Bootstrap

1. Create Fly API token.
2. Create/identify Fly app target.

### Required Env

- `FLY_API_TOKEN`
- `FLY_APP_NAME`

### Verify

```bash
runfabric doctor -c examples/hello-http/runfabric.fly-machines.yml
```

## IBM OpenWhisk

### Bootstrap

1. Create IBM Cloud API key.
2. Select region and namespace.

### Required Env

- `IBM_CLOUD_API_KEY`
- `IBM_CLOUD_REGION`
- `IBM_CLOUD_NAMESPACE`

### Verify

```bash
runfabric doctor -c examples/hello-http/runfabric.ibm-openwhisk.yml
```

## Notes

- `runfabric doctor` validates only providers listed in your config.
- For package users, env-based credentials are first-class and do not require CI pipelines.
