# Quickstart: Hello World

This guide helps you run the `hello-http` example end-to-end with minimal setup.

## What You Will Build
- A Hello World HTTP function from `examples/hello-http/src/index.ts`
- A single-provider deployment flow using `cloudflare-workers`

## Prerequisites
- Node.js `>= 20`
- pnpm (via Corepack)
- Cloudflare credentials:
  - `CLOUDFLARE_API_TOKEN`
  - `CLOUDFLARE_ACCOUNT_ID`

## 1. Install Dependencies
From repo root:

```bash
corepack enable
corepack prepare pnpm@10.5.2 --activate
pnpm install
```

## 2. Set Credentials
```bash
export CLOUDFLARE_API_TOKEN="your-token"
export CLOUDFLARE_ACCOUNT_ID="your-account-id"
```

Optional for real Cloudflare API deploy:
```bash
export RUNFABRIC_CLOUDFLARE_REAL_DEPLOY=1
```

If `RUNFABRIC_CLOUDFLARE_REAL_DEPLOY` is not set, `runfabric deploy` runs in simulated mode and writes a deployment receipt without provisioning cloud resources.

Alternative using `.env`:
```bash
cp .env.example .env
# set CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID in .env
set -a
source .env
set +a
```

## 3. Run Doctor
```bash
pnpm run runfabric -- doctor -c examples/hello-http/runfabric.quickstart.yml
```

You should see `doctor checks passed`.

## 4. Plan
```bash
pnpm run runfabric -- plan -c examples/hello-http/runfabric.quickstart.yml
```

Optional stage override:
```bash
pnpm run runfabric -- plan -c examples/hello-http/runfabric.quickstart.yml --stage prod
```

## 5. Build
```bash
pnpm run runfabric -- build -c examples/hello-http/runfabric.quickstart.yml
```

## 6. Deploy
```bash
pnpm run runfabric -- deploy -c examples/hello-http/runfabric.quickstart.yml
```

Expected output includes an endpoint like:
- simulated mode: `cloudflare-workers: https://hello-http.workers.dev`
- real API mode: `cloudflare-workers: https://<script>.<account-subdomain>.workers.dev`

## 7. Verify Deployment Receipt
`runfabric` writes a local deployment receipt:

```bash
cat examples/hello-http/.runfabric/deploy/cloudflare-workers/deployment.json
```

It also writes stage-aware provider state:
```bash
cat examples/hello-http/.runfabric/state/hello-http/default/cloudflare-workers.state.json
```

## Notes
- Cloudflare is the first provider with an optional real API deploy path. Other providers currently use simulated deployment receipts.
- For provider-wise credential lists and other ways to pass creds, see `docs/CREDENTIALS.md`.
- If you install `runfabric` as a package, run the same commands directly (replace `pnpm run runfabric --` with `runfabric`).
- For one-provider configs across all supported providers, see `examples/hello-http/PROVIDERS.md`.
- Additional lifecycle commands:
  - `runfabric package -c <config>` for artifact packaging
  - `runfabric deploy-function <name> -c <config>` for function-level deploy
  - `runfabric remove -c <config>` for cleanup
