# Hello HTTP Provider Examples

All example configs use the same handler (`src/index.ts`) and HTTP trigger.

## Config Files
- `runfabric.quickstart.yml`: cloudflare-only minimal first run
- `runfabric.aws-lambda.yml`
- `runfabric.gcp-functions.yml`
- `runfabric.azure-functions.yml`
- `runfabric.cloudflare-workers.yml`
- `runfabric.vercel.yml`
- `runfabric.netlify.yml`
- `runfabric.alibaba-fc.yml`
- `runfabric.digitalocean-functions.yml`
- `runfabric.fly-machines.yml`
- `runfabric.ibm-openwhisk.yml`
- `runfabric.compose.yml` (compose orchestration sample)

## Run Any Provider Example
From repo root:

```bash
pnpm run runfabric -- doctor -c examples/hello-http/runfabric.<provider>.yml
pnpm run runfabric -- plan -c examples/hello-http/runfabric.<provider>.yml
pnpm run runfabric -- build -c examples/hello-http/runfabric.<provider>.yml
pnpm run runfabric -- package -c examples/hello-http/runfabric.<provider>.yml
pnpm run runfabric -- deploy -c examples/hello-http/runfabric.<provider>.yml
pnpm run runfabric -- remove -c examples/hello-http/runfabric.<provider>.yml
```

Replace `<provider>` with one of:
- `aws-lambda`
- `gcp-functions`
- `azure-functions`
- `cloudflare-workers`
- `vercel`
- `netlify`
- `alibaba-fc`
- `digitalocean-functions`
- `fly-machines`
- `ibm-openwhisk`

## Credentials
Set provider credentials before running commands.

See:
- `docs/CREDENTIALS.md`
