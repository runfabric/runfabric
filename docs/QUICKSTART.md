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
pnpm run runfabric -- init --dir ./my-api
```

Interactive `init` prompts for:

- template (`api`, `worker`, `queue`, `cron`)
- provider
- language (`ts` or `js`)

It also creates:

- `package.json` with `@runfabric/core`
- `call:local` script that runs `runfabric call-local -c runfabric.yml --serve --watch`

Non-interactive example:

```bash
pnpm run runfabric -- init --dir ./my-api --template api --provider aws-lambda --lang ts --skip-install
```

Run lifecycle:

```bash
pnpm run runfabric -- doctor -c ./my-api/runfabric.yml
pnpm run runfabric -- plan -c ./my-api/runfabric.yml
pnpm run runfabric -- build -c ./my-api/runfabric.yml
pnpm run runfabric -- deploy -c ./my-api/runfabric.yml
pnpm run runfabric -- logs --provider aws-lambda
```

Run local provider-mimic server from your scaffolded project:

```bash
cd my-api
pnpm run call:local
curl -i http://127.0.0.1:8787/hello
# stop server: Ctrl+C or type 'exit' and press Enter
pnpm run call:local -- --port 3000
curl -i http://127.0.0.1:3000/hello

# one-shot (non-server) invocation still available:
pnpm run call:local -- --provider aws-lambda --method GET --path /hello
pnpm run call:local -- --provider aws-lambda --event ./event.json
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

## Framework Handler Wrappers

You can reuse existing framework apps with `UniversalHandler` using `@runfabric/runtime-node`:

```ts
import type { UniversalHandler } from "@runfabric/core";
import { createHandler } from "@runfabric/runtime-node";

// Auto-detects Nest app (getHttpAdapter), Fastify instance (inject), or Express app function:
export const handler: UniversalHandler = createHandler(appOrFastifyOrNestApp);
```

More handler examples:

- `docs/HANDLER_SCENARIOS.md`
- `examples/handler-scenarios/README.md`
