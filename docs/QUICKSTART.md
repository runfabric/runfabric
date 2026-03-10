# Quickstart

This guide gives two easy onboarding paths:

1. run existing Hello World example
2. scaffold a new project with `runfabric init`

## Prerequisites

- Node.js `>= 20`
- pnpm via Corepack

Install:

```bash
corepack enable
corepack prepare pnpm@10.5.2 --activate
pnpm install
```

## Path A: Use Existing Hello HTTP Example

Use `examples/hello-http/runfabric.quickstart.yml`.

Set Cloudflare credentials:

```bash
export CLOUDFLARE_API_TOKEN="your-token"
export CLOUDFLARE_ACCOUNT_ID="your-account-id"
```

Run:

```bash
pnpm run runfabric -- doctor -c examples/hello-http/runfabric.quickstart.yml
pnpm run runfabric -- plan -c examples/hello-http/runfabric.quickstart.yml
pnpm run runfabric -- build -c examples/hello-http/runfabric.quickstart.yml
pnpm run runfabric -- deploy -c examples/hello-http/runfabric.quickstart.yml
```

Optional real Cloudflare API deployment:

```bash
export RUNFABRIC_CLOUDFLARE_REAL_DEPLOY=1
pnpm run runfabric -- deploy -c examples/hello-http/runfabric.quickstart.yml
```

## Path B: Scaffold New Project

Create API template:

```bash
pnpm run runfabric -- init --template api --dir ./my-api --provider aws-lambda
```

Available templates:

- `api`
- `worker`
- `queue`
- `cron`

Run lifecycle:

```bash
pnpm run runfabric -- doctor -c ./my-api/runfabric.yml
pnpm run runfabric -- plan -c ./my-api/runfabric.yml
pnpm run runfabric -- build -c ./my-api/runfabric.yml
pnpm run runfabric -- deploy -c ./my-api/runfabric.yml
pnpm run runfabric -- logs --provider aws-lambda
```

## Local State And Receipts

After deploy:

- receipt: `.runfabric/deploy/<provider>/deployment.json`
- state: `.runfabric/state/<service>/<stage>/<provider>.state.json`

## Real Deploy Mode For Other Providers

Default deploy is simulated. To enable real mode:

- `RUNFABRIC_REAL_DEPLOY=1` globally, or
- `RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1` per provider

Then set provider deploy command env var returning JSON, e.g.:

- `RUNFABRIC_AWS_DEPLOY_CMD`
- `RUNFABRIC_GCP_DEPLOY_CMD`
- `RUNFABRIC_AZURE_DEPLOY_CMD`

Full credential and command matrix: `docs/CREDENTIALS.md`.
