# Quickstart

This guide gives two easy onboarding paths:

1. run existing Hello World example
2. scaffold a new project with `runfabric init`

## Prerequisites

- Node.js `>= 20`
- pnpm via Corepack
- Current beta runtime target: `runtime: nodejs`

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

Migrate existing `serverless.yml` (best-effort bootstrap):

```bash
pnpm run runfabric -- migrate --input ./serverless.yml --output ./runfabric.yml --json
```

Run lifecycle:

```bash
pnpm run runfabric -- doctor -c ./my-api/runfabric.yml
pnpm run runfabric -- plan -c ./my-api/runfabric.yml
pnpm run runfabric -- build -c ./my-api/runfabric.yml
pnpm run runfabric -- package -c ./my-api/runfabric.yml --function api
pnpm run runfabric -- deploy -c ./my-api/runfabric.yml
pnpm run runfabric -- deploy -c ./my-api/runfabric.yml --function api
pnpm run runfabric -- logs --provider aws-lambda
pnpm run runfabric -- traces --provider aws-lambda --json
pnpm run runfabric -- metrics --provider aws-lambda --json
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

For TypeScript entries, `call-local` now runs an initial `tsc -p tsconfig.json` automatically when no built handler artifact is found (published CLI mode). Keep `typescript` installed in your project dev dependencies.

Or use the unified dev loop:

```bash
pnpm run runfabric -- dev -c ./my-api/runfabric.yml --preset http --watch
pnpm run runfabric -- dev -c ./my-api/runfabric.yml --preset queue --once
pnpm run runfabric -- dev -c ./my-api/runfabric.yml --preset storage --once
```

## State Backends And Receipts

After deploy:

- receipt: `.runfabric/deploy/<provider>/deployment.json`
- state (`local` backend): `.runfabric/state/<service>/<stage>/<provider>.state.json`
- state (`postgres` backend): persisted in configured Postgres table
- state (`s3|gcs|azblob` backends): persisted as objects under configured prefix

Example state config in `runfabric.yml`:

```yaml
state:
  backend: s3
  keyPrefix: runfabric/state
  lock:
    enabled: true
    timeoutSeconds: 30
  s3:
    bucket: my-runfabric-state
    region: us-east-1
    keyPrefix: runfabric/state
```

State operations:

```bash
pnpm run runfabric -- state list -c ./my-api/runfabric.yml --json
pnpm run runfabric -- state pull -c ./my-api/runfabric.yml --provider aws-lambda --json
pnpm run runfabric -- state backup -c ./my-api/runfabric.yml --out ./.runfabric/backup/state.json --json
pnpm run runfabric -- state restore -c ./my-api/runfabric.yml --file ./.runfabric/backup/state.json --json
pnpm run runfabric -- state reconcile -c ./my-api/runfabric.yml --json
pnpm run runfabric -- state force-unlock -c ./my-api/runfabric.yml --provider aws-lambda --json
pnpm run runfabric -- state migrate -c ./my-api/runfabric.yml --from local --to postgres --json
```

## Real Deploy Mode For Other Providers

Default deploy is simulated. To enable real mode:

- `RUNFABRIC_REAL_DEPLOY=1` globally, or
- `RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1` per provider

Then set provider deploy command env var returning JSON, e.g.:

- `RUNFABRIC_AWS_DEPLOY_CMD`
- `RUNFABRIC_GCP_DEPLOY_CMD`
- `RUNFABRIC_AZURE_DEPLOY_CMD`

Full credential and command matrix: `docs/CREDENTIALS.md`.
State backend credentials/auth/IAM: `docs/STATE_BACKENDS.md`.

## AWS Queue/Storage/IAM Example

Example `runfabric.yml` using queue + storage triggers, AWS IAM statements, and function-level env:

```yaml
service: media-worker
runtime: nodejs
entry: src/index.ts

providers:
  - aws-lambda

triggers:
  - type: queue
    queue: arn:aws:sqs:us-east-1:123456789012:media-jobs
    batchSize: 10
    maximumBatchingWindowSeconds: 5
    functionResponseType: ReportBatchItemFailures
  - type: storage
    bucket: media-uploads
    events:
      - s3:ObjectCreated:*
    prefix: incoming/
    suffix: .jpg

extensions:
  aws-lambda:
    region: us-east-1
    iam:
      role:
        statements:
          - effect: Allow
            actions:
              - s3:GetObject
              - s3:PutObject
            resources:
              - arn:aws:s3:::media-uploads/*

functions:
  - name: process-media
    entry: src/process-media.ts
    env:
      BUCKET: media-uploads
```

When `RUNFABRIC_AWS_REAL_DEPLOY=1`, deploy passes these JSON payload env vars to `RUNFABRIC_AWS_DEPLOY_CMD`:

- `RUNFABRIC_AWS_QUEUE_EVENT_SOURCES_JSON`
- `RUNFABRIC_AWS_STORAGE_EVENTS_JSON`
- `RUNFABRIC_AWS_IAM_ROLE_STATEMENTS_JSON`
- `RUNFABRIC_FUNCTION_ENV_JSON`

Additional AWS payloads for P3/P4 schema:

- `RUNFABRIC_AWS_EVENTBRIDGE_RULES_JSON`
- `RUNFABRIC_AWS_RESOURCE_ADDRESSES_JSON`
- `RUNFABRIC_AWS_WORKFLOW_ADDRESSES_JSON`
- `RUNFABRIC_AWS_SECRET_REFERENCES_JSON`

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
