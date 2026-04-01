# Quickstart

This guide gives two easy onboarding paths:

1. run existing Hello World example
2. scaffold a new project with `runfabric init`

**CLI:** Use the Go-built binary (`make build` → `./bin/runfabric`) or the npm package (`npx @runfabric/cli` / `pnpm run runfabric --`). All examples below use `runfabric`; if you built from source, substitute `./bin/runfabric`.

## Quick navigation

- **I want the fastest smoke test** → Path A (existing hello example)
- **I want to start a new project** → Path B (`runfabric init`)
- **I want to add features later** → “Adding a new function (generate)” and [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md)

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

Use `examples/node/hello-http/runfabric.cloudflare-workers.yml`.

Set Cloudflare credentials:

```bash
export CLOUDFLARE_API_TOKEN="your-token"
export CLOUDFLARE_ACCOUNT_ID="your-account-id"
```

Run:

```bash
pnpm run runfabric -- doctor -c examples/node/hello-http/runfabric.cloudflare-workers.yml
pnpm run runfabric -- plan -c examples/node/hello-http/runfabric.cloudflare-workers.yml
pnpm run runfabric -- build -c examples/node/hello-http/runfabric.cloudflare-workers.yml
pnpm run runfabric -- deploy -c examples/node/hello-http/runfabric.cloudflare-workers.yml
```

Optional real Cloudflare API deployment:

```bash
export RUNFABRIC_CLOUDFLARE_REAL_DEPLOY=1
pnpm run runfabric -- deploy -c examples/node/hello-http/runfabric.cloudflare-workers.yml
```

## Path B: Scaffold New Project

Create API template:

```bash
pnpm run runfabric -- init --dir ./my-api
```

Default service name is derived from the target directory (`my-api` here). Use `--service` to override.

Interactive `init` prompts for:

- template (`http`, `queue`, `cron`, `storage`, `eventbridge`, `pubsub`)
- provider
- optional secret manager plugin (`none` or a discovered `kind=secret-manager` plugin ID)
- state backend (`local`, `postgres`, `s3`, `gcs`, `azblob`) with default `local`
- language (`ts` or `js`)

Interactive picker UX:

- grouped options for templates/providers/state backends
- type to filter options in place
- keyboard controls: `Up/Down` to move, `Enter` to select, `Backspace` to edit filter, `Esc` to clear filter
- template list is filtered by selected provider capabilities

Template scope note:

- `init` only includes templates with at least one provider capability match: `http|queue|cron|storage|eventbridge|pubsub`.
- `kafka` and `rabbitmq` remain valid trigger schema types but are hidden from `init` until at least one provider reports support.

Provider IDs (copy/paste):

- `aws-lambda`
- `gcp-functions`
- `azure-functions`
- `kubernetes`
- `cloudflare-workers`
- `vercel`
- `netlify`
- `alibaba-fc`
- `digitalocean-functions`
- `fly-machines`
- `ibm-openwhisk`

It also creates:

- `package.json` with zero runtime dependencies and, for TypeScript scaffolds, devDependencies plus a `build` script
- `call:local` script:
  - TypeScript scaffolds: runs initial `tsc` build and then runs `tsc --watch` in parallel with `runfabric invoke local -c runfabric.yml --serve --watch`
  - JavaScript scaffolds: runs `runfabric invoke local -c runfabric.yml --serve --watch`
- `.env.example` with provider and selected state backend variables
- project-scoped random state prefix for object backends (`s3`, `gcs`, `azblob`)

> **Dependencies note**: The scaffold uses plain handler exports with zero runtime dependencies, so handlers are invocable immediately without waiting for SDK or provider packages to be published. Runtime SDK (`@runfabric/sdk`) and provider adapters can be added optionally when available and needed for advanced features.

Copy and load `.env.example` before deploy:

```bash
cd my-api
cp .env.example .env
set -a
source .env
set +a
```

Non-interactive example:

```bash
pnpm run runfabric -- init --dir ./my-api --template http --provider aws-lambda --lang ts --skip-install
```

Non-interactive with explicit state backend:

```bash
pnpm run runfabric -- init --dir ./my-api --template http --provider aws-lambda --state-backend s3 --lang ts --skip-install
```

Non-interactive with secret manager selection:

```bash
pnpm run runfabric -- init --dir ./my-api --template http --provider aws-lambda --secret-manager vault-secret-manager --lang ts --skip-install
```

If you used `--skip-install`, install project dependencies manually:

```bash
cd my-api
pnpm install
pnpm run build
```

### Adding a new function (generate)

Inside an existing project, add a function without hand-editing `runfabric.yml`:

```bash
runfabric generate function hello --trigger http --route GET:/hello
runfabric generate worker worker --queue-name my-queue
runfabric generate function cron-job --trigger cron --schedule "rate(5 minutes)"
```

Interactive flow (prompts for name, language, trigger options, entry path, and confirmation):

```bash
runfabric generate function --interactive
```

Interactive resource/addon flows:

```bash
runfabric generate resource --interactive
runfabric generate addon --interactive
```

Automation-safe non-interactive mode (no prompts, deterministic behavior):

```bash
runfabric generate function hello --trigger http --route GET:/hello --no-interactive
runfabric generate resource db --type database --connection-env DATABASE_URL --no-interactive
```

Use `--dry-run` to preview, `--force` to overwrite an existing handler file. See [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md).

Migrate an existing `serverless.yml` by following [MIGRATION.md](MIGRATION.md) and then validate with `doctor` + `plan`.

Run lifecycle:

```bash
pnpm run runfabric -- doctor -c ./my-api/runfabric.yml
pnpm run runfabric -- plan -c ./my-api/runfabric.yml
pnpm run runfabric -- build -c ./my-api/runfabric.yml
pnpm run runfabric -- package -c ./my-api/runfabric.yml --function api
pnpm run runfabric -- deploy -c ./my-api/runfabric.yml
pnpm run runfabric -- deploy -c ./my-api/runfabric.yml --rollback-on-failure
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
pnpm run call:local -- --serve --event ./event.template.json

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
# additional presets: cron | eventbridge | pubsub | kafka | rabbitmq
```

Compose-style multi-service orchestration:

```bash
# validate ordering + planning across all services
pnpm run runfabric -- compose plan -f ./runfabric.compose.yml --concurrency 2

# deploy independent services in parallel by dependency level
pnpm run runfabric -- compose deploy -f ./runfabric.compose.yml --concurrency 4

# optional rollback semantics mirror deploy command behavior
pnpm run runfabric -- compose deploy -f ./runfabric.compose.yml --rollback-on-failure

# remove in reverse dependency order (optionally scoped to one provider)
pnpm run runfabric -- compose remove -f ./runfabric.compose.yml --provider aws-lambda
```

Compose deploy exports upstream endpoints as environment variables for downstream service workflows:

- `RUNFABRIC_OUTPUT_<SERVICE>_<PROVIDER>_ENDPOINT`

Router DNS/LB operations for multi-target `fabric` deployments:

```bash
# deploy all fabric targets, then sync DNS/LB in one flow
pnpm run runfabric -- router deploy -c ./my-api/runfabric.yml --sync-dns

# drift report (dry-run)
pnpm run runfabric -- router dns-reconcile -c ./my-api/runfabric.yml

# apply reconcile
pnpm run runfabric -- router dns-reconcile -c ./my-api/runfabric.yml --apply

# restore from previous applied snapshot (last-known-good)
pnpm run runfabric -- router dns-restore -c ./my-api/runfabric.yml

# inspect sync history trend + latest operation metadata
pnpm run runfabric -- router dns-history -c ./my-api/runfabric.yml

# simulate weighted routing locally (no provider API calls)
pnpm run runfabric -- router simulate -c ./my-api/runfabric.yml --requests 500

# chaos/failover verification (single-endpoint-down + all-down scenarios)
pnpm run runfabric -- router chaos-verify -c ./my-api/runfabric.yml

# progressive canary shift to one endpoint
pnpm run runfabric -- router dns-shift -c ./my-api/runfabric.yml --provider aws-us --percent 20 --dry-run
```

## State Backends And Receipts

After deploy:

- receipt: `.runfabric/deploy/<provider>/deployment.json`
- state (`local` backend): `.runfabric/state/<service>/<stage>/<provider>.state.json`
- state (`postgres` backend): persisted in configured Postgres table
- state (`s3|gcs|azblob` backends): persisted as objects under configured prefix

Example state config in `runfabric.yml`:

```yaml
backend:
  kind: s3
  s3Bucket: my-runfabric-state
  s3Prefix: runfabric/state
  lockTable: my-runfabric-locks
```

Dynamic environment bindings in `runfabric.yml` are supported:

```yaml
service: ${env:RUNFABRIC_SERVICE_NAME,my-api}
backend:
  kind: s3
  s3Bucket: ${env:RUNFABRIC_STATE_S3_BUCKET}
  s3Prefix: ${env:RUNFABRIC_STATE_S3_PREFIX,runfabric/state}
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
- `kubernetes`: `kubectl` command contract
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
Credentials and state backends: [CREDENTIALS.md](CREDENTIALS.md), [STATE_BACKENDS.md](STATE_BACKENDS.md).
Example validation checklist: `docs/EXAMPLE_VALIDATION.md`.

## AWS Queue/Storage/IAM Example

Example `runfabric.yml` using queue + storage triggers, AWS IAM statements, and function-level env:

```yaml
service: media-worker
provider:
  name: aws-lambda
  runtime: nodejs

functions:
  - name: process-media
    entry: src/process-media.ts
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
    env:
      BUCKET: media-uploads

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

You can reuse existing framework apps with `UniversalHandler` using `@runfabric/sdk`:

```ts
import type { UniversalHandler } from "@runfabric/sdk";
import { createHandler } from "@runfabric/sdk";

// Auto-detects Nest app (getHttpAdapter), Fastify instance (inject), or Express app function:
export const handler: UniversalHandler = createHandler(appOrFastifyOrNestApp);
```

More handler examples:

- `packages/node/sdk/README.md` — Handler patterns and framework adapters
- `examples/handler-scenarios/README.md`
