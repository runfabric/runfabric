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

Default service name is derived from the target directory (`my-api` here). Use `--service` to override.

Interactive `init` prompts for:

- template (`api`, `worker`, `queue`, `cron`)
- provider
- state backend (`local`, `postgres`, `s3`, `gcs`, `azblob`) with default `local`
- language (`ts` or `js`)

Interactive picker UX:

- grouped options for templates/providers/state backends
- type to filter options in place
- keyboard controls: `Up/Down` to move, `Enter` to select, `Backspace` to edit filter, `Esc` to clear filter
- template list is filtered by selected provider capabilities

Template scope note:

- `init` currently scaffolds only `api|worker|queue|cron`.
- For `storage|eventbridge|pubsub` scenarios, scaffold from `worker` then edit `triggers` in `runfabric.yml`.

Provider IDs (copy/paste):

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

It also creates:

- `package.json` with `@runfabric/core` and the selected provider adapter dependency
- `call:local` script that runs `runfabric call-local -c runfabric.yml --serve --watch`
- `.env.example` with provider and selected state backend variables
- project-scoped random state prefix for object backends (`s3`, `gcs`, `azblob`)

Copy and load `.env.example` before deploy:

```bash
cd my-api
cp .env.example .env
set -a
source .env
set +a
```

Provider adapters are loaded dynamically. Install only providers used by the project
(init installs the selected provider adapter unless `--skip-install` is used).

Non-interactive example:

```bash
pnpm run runfabric -- init --dir ./my-api --template api --provider aws-lambda --lang ts --skip-install
```

Non-interactive with explicit state backend:

```bash
pnpm run runfabric -- init --dir ./my-api --template api --provider aws-lambda --state-backend s3 --lang ts --skip-install
```

If you used `--skip-install`, install project dependencies manually:

```bash
cd my-api
pnpm add @runfabric/core @runfabric/provider-aws-lambda
pnpm add -D typescript @types/node
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

# non-http local simulation via explicit event payload:
pnpm run call:local -- --provider aws-lambda --event ./event.queue.json
pnpm run call:local -- --provider aws-lambda --event ./event.storage.json
pnpm run call:local -- --provider aws-lambda --event ./event.eventbridge.json
pnpm run call:local -- --provider gcp-functions --event ./event.pubsub.json
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

Dynamic environment bindings in `runfabric.yml` are supported:

```yaml
service: ${env:RUNFABRIC_SERVICE_NAME,my-api}
state:
  backend: s3
  s3:
    bucket: ${env:RUNFABRIC_STATE_S3_BUCKET}
    region: ${env:AWS_REGION,us-east-1}
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

## Real Deploy Mode

Default deploy is simulated. To enable real mode:

- `RUNFABRIC_REAL_DEPLOY=1` globally, or
- `RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1` per provider

Built-in real deployers are used by default:

- `aws-lambda`: AWS SDK deployer (requires `RUNFABRIC_AWS_LAMBDA_ROLE_ARN`)
- `cloudflare-workers`: direct Cloudflare API deployer
- other providers: built-in provider CLI command contracts

One-time AWS role bootstrap for real deploy:

```bash
aws iam create-role \
  --role-name runfabric-lambda-exec \
  --assume-role-policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}'

aws iam attach-role-policy \
  --role-name runfabric-lambda-exec \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

export RUNFABRIC_AWS_REAL_DEPLOY=1
export RUNFABRIC_AWS_LAMBDA_ROLE_ARN="$(aws iam get-role --role-name runfabric-lambda-exec --query 'Role.Arn' --output text)"
```

If AWS returns assume-role errors right after role creation, wait 20-60 seconds and retry deploy.

For real AWS deployments, keep `RUNFABRIC_AWS_REAL_DEPLOY=1` set when running `runfabric remove`; otherwise cloud deletion is blocked and remove fails fast with guidance.

Optional override hooks:

- `RUNFABRIC_<PROVIDER>_DEPLOY_CMD` (deploy command must return JSON)
- `RUNFABRIC_<PROVIDER>_DESTROY_CMD`

Full credential and command matrix: `docs/CREDENTIALS.md`.
State backend credentials/auth/IAM: `docs/STATE_BACKENDS.md`.
Combined quick matrix: `docs/CREDENTIALS_MATRIX.md`.
Example validation checklist: `docs/EXAMPLE_VALIDATION.md`.

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
    roleArn: arn:aws:iam::123456789012:role/runfabric-lambda-role
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

When `RUNFABRIC_AWS_REAL_DEPLOY=1` and `RUNFABRIC_AWS_DEPLOY_CMD` is set (optional override), deploy passes these JSON payload env vars to that command:

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
