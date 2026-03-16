# runfabric.yml Reference

Canonical config reference for the current release train. Aligned with [upstream RUNFABRIC_YML_REFERENCE](https://github.com/runfabric/runfabric/blob/main/docs/RUNFABRIC_YML_REFERENCE.md). In this repo the Go engine normalizes reference format into `provider`/`backend`/`functions`. **JSON Schema:** [schemas/runfabric.schema.json](../schemas/runfabric.schema.json).

## Minimum Example

```yaml
service: hello-api
runtime: nodejs
entry: src/index.ts

providers:
  - aws-lambda

triggers:
  - type: http
    method: GET
    path: /hello
```

## Top-Level Fields

- `service` (`string`, required)
- `runtime` (`string`, required)
- `entry` (`string`, required)
- `providers` (`string[]`, required)
- `triggers` (`trigger[]`, required)
- `functions` (`function[]`, optional)
- `hooks` (`string[]`, optional)
- `env` (`Record<string,string>`, optional)
- `resources` (`object`, optional) — Managed resource binding: declare DB/cache and inject `DATABASE_URL`, `REDIS_URL`, etc. into function env at deploy. See [Managed resource binding](#managed-resource-binding).
- `secrets` (`Record<string,string>`, optional; value format `secret://<ref>`)
- `workflows` (`workflow[]`, optional)
- `params` (`Record<string,string>`, optional)
- `extensions` (`object`, optional)
- `deploy` (`object`, optional)
- `state` (`object`, optional)
- `stages` (`Record<string, override>`, optional)
- `providerOverrides` (`Record<string, provider>`, optional) — Multi-cloud: named provider configs. Use with `runfabric deploy --provider <key>`, `runfabric plan --provider <key>`, `runfabric remove --provider <key>`. Key is a logical name (e.g. `aws`, `gcp`); value is the same shape as `provider` (name, runtime, region).
- `layers` (`Record<string, layer>`, optional) — First-class layer declarations. Key = logical name; value has `arn` (required), optional `name`/`version`. See [First-class layers](#first-class-layers) below.
- `addons` (`Record<string, addon>`, optional) — Add-on declarations (marketplace-style). See [Add-ons](#add-ons) and `runfabric addons list`.
- `addonCatalogUrl` (`string`, optional) — URL to fetch addon catalog entries (JSON array); merged with built-in when running `runfabric addons list`.
- `fabric` (`object`, optional) — Runtime fabric for active-active deploy, health checks, and failover/latency routing. Requires `providerOverrides`. See [Runtime fabric](#runtime-fabric).
- `logs` (`object`, optional) — Local log file source for `runfabric logs`. When set, logs are read from provider (e.g. CloudWatch) and merged with lines from local files. See [Logs](#logs).
- `app` (`string`, optional) — Application or project group name for dashboard/UI grouping.
- `org` (`string`, optional) — Organization or tenant identifier for multi-tenant dashboards.
- `build` (`object`, optional) — Build-step ordering. See [Build order](#build-order).
- `alerts` (`object`, optional) — Optional alerting config (webhook, Slack, triggers). See [Alerts](#alerts).

## Multi-cloud (providerOverrides)

When you want one `runfabric.yml` to target multiple providers (e.g. AWS and GCP), define `providerOverrides` and pass `--provider <key>` on deploy, plan, and remove:

```yaml
service: my-api
provider:
  name: aws-lambda
  runtime: nodejs
  region: us-east-1

providerOverrides:
  aws:
    name: aws-lambda
    runtime: nodejs
    region: us-east-1
    backend:                    # optional: per-provider state backend (when using --provider aws)
      kind: s3
      s3Bucket: my-aws-bucket
  gcp:
    name: gcp-functions
    runtime: nodejs
    region: us-central1
    backend:                    # optional: e.g. gcs for GCP
      kind: gcs

# ... functions, triggers, etc.
```

Then run e.g. `runfabric deploy --provider aws --stage prod` or `runfabric deploy --provider gcp --stage prod`. Without `--provider`, the top-level `provider` block is used. When a provider override includes `backend`, that backend is used for state (receipts, locks) when `--provider <key>` is set. Invoke, logs, metrics, and traces also accept `--provider` for multi-cloud.

## First-class layers

Define layers once and reference them by name from functions (providers that support layers, e.g. AWS Lambda, resolve the ARN):

```yaml
layers:
  node-deps:
    arn: "arn:aws:lambda:us-east-1:123456789012:layer:node-deps:1"
    name: node-deps
    version: "1"
  custom:
    arn: "${env:LAMBDA_LAYER_ARN}"
    version: "${env:LAYER_VERSION}"   # optional: set from CI (e.g. package-lock hash)

functions:
  api:
    handler: src/handler.default
    layers: ["node-deps", "custom"]
```

Entries in `functions.<name>.layers` can be logical names (keys in top-level `layers`) or literal layer ARNs.

**Versioning on dependency change:** Use `version` with an env var (e.g. `version: "${env:LAYER_VERSION}"`) and set that in CI from a hash of `package-lock.json` or `requirements.txt` so layer ARNs/versions track dependency changes. Resolve runs after env is set, so the same config works across environments.

**Other providers:** Layers are applied by AWS Lambda today. Other providers (GCP, Azure, etc.) preserve the `layers` config but do not apply it; use provider-specific mechanisms (e.g. build env, separate artifacts) where needed.

## Dynamic Env Bindings

String values can resolve environment variables using:

- `${env:VAR_NAME}`
- `${env:VAR_NAME,default-value}`

Example:

```yaml
service: ${env:RUNFABRIC_SERVICE_NAME,my-service}
runtime: nodejs
entry: src/index.ts

providers:
  - aws-lambda

state:
  backend: s3
  s3:
    bucket: ${env:RUNFABRIC_STATE_S3_BUCKET}
    region: ${env:AWS_REGION,us-east-1}

triggers:
  - type: http
    method: GET
    path: /hello
```

If `${env:VAR_NAME}` is used without a default and the variable is missing, config parsing fails with an explicit error.

## Deploy Policy

Single-function deploy: use `runfabric deploy --function <name>`, `runfabric deploy fn <name>`, `runfabric deploy function <name>`, or `runfabric deploy-function <name>`.

```yaml
deploy:
  rollbackOnFailure: true   # optional
  strategy: all-at-once     # optional: all-at-once (default), canary, blue-green
  canaryPercent: 10         # 0-100 when strategy: canary (provider-specific traffic shift)
  canaryIntervalMinutes: 5  # minutes before full shift when strategy: canary (optional)
  healthCheck:             # optional post-deploy HTTP GET
    enabled: true
    url: ""                # empty = use deployed URL from receipt (ServiceURL, url, ApiUrl)
  scaling:                 # optional provider-level defaults (overridden per function)
    reservedConcurrency: 10
    provisionedConcurrency: 0
```

- **strategy**: `all-at-once` (default), `canary`, or `blue-green`. **AWS Lambda** implements blue-green and canary: publishes a new version, uses alias `live` for API Gateway so traffic switches to the new version; canary optionally waits `canaryIntervalMinutes` before switching. Use `healthCheck.enabled: true` and `rollbackOnFailure` for safe rollback. Other providers may implement strategy per provider.
- **canaryPercent** / **canaryIntervalMinutes**: When `strategy: canary`, optional hint for gradual traffic shift (0-100% and wait time). On AWS, `canaryIntervalMinutes` is the delay before the alias is updated to the new version; behavior is provider-dependent elsewhere.
- **healthCheck**: If `enabled: true`, after a successful deploy the CLI runs an HTTP GET to the deployed URL (or `url` if set). On non-2xx response and when rollback is enabled, the deployment is removed and an error is returned.
- **scaling**: Defaults for `reservedConcurrency` and `provisionedConcurrency` (e.g. AWS Lambda). Per-function values override these.

Per-function scaling (and layers) in `functions`:

```yaml
functions:
  api:
    handler: src/handler.default
    reservedConcurrency: 5      # AWS: reserved concurrency
    provisionedConcurrency: 1   # AWS: provisioned concurrency (use with version/alias)
    layers: ["node-deps"]       # refs to top-level layers.* or literal ARNs
```

Stage override:

```yaml
stages:
  prod:
    deploy:
      rollbackOnFailure: true
```

Behavior precedence for rollback-on-failure:

1. CLI flag (`deploy --rollback-on-failure` or `--no-rollback-on-failure`)
2. `runfabric.yml` deploy policy (`deploy.rollbackOnFailure`)
3. Legacy env toggle (`RUNFABRIC_ROLLBACK_ON_FAILURE`)

## Trigger Types

### HTTP

```yaml
- type: http
  method: GET
  path: /hello
```

### Cron

```yaml
- type: cron
  schedule: "*/5 * * * *"
  timezone: UTC # optional
```

### Queue

```yaml
- type: queue
  queue: arn:aws:sqs:us-east-1:123456789012:jobs
  batchSize: 10 # optional
  maximumBatchingWindowSeconds: 5 # optional
  maximumConcurrency: 2 # optional
  enabled: true # optional
  functionResponseType: ReportBatchItemFailures # optional
```

### Storage

```yaml
- type: storage
  bucket: uploads
  events:
    - s3:ObjectCreated:*
  prefix: incoming/ # optional
  suffix: .jpg # optional
  existingBucket: true # optional
```

### EventBridge / PubSub / Kafka / RabbitMQ

```yaml
- type: eventbridge
  pattern:
    source:
      - app.source
  bus: default # optional

- type: pubsub
  topic: jobs
  subscription: jobs-sub # optional

- type: kafka
  brokers:
    - kafka:9092
  topic: events
  groupId: runfabric

- type: rabbitmq
  queue: jobs
  exchange: app-exchange # optional
  routingKey: app.jobs # optional
```

## Function Overrides

```yaml
functions:
  - name: api
    entry: src/api.ts
    runtime: nodejs # optional override
    triggers:
      - type: http
        method: POST
        path: /api
    env:
      FEATURE_FLAG: "1"
```

## AWS Extension Example

```yaml
extensions:
  aws-lambda:
    region: us-east-1
    stage: dev
    roleArn: arn:aws:iam::123456789012:role/runfabric-lambda-role # required for internal AWS real deploy
    functionName: my-service-dev # optional override
    runtime: nodejs20.x # optional runtime override for internal AWS real deploy
    iam:
      role:
        statements:
          - effect: Allow
            actions:
              - s3:GetObject
            resources:
              - arn:aws:s3:::uploads/*
```

## Kubernetes Extension Example

```yaml
extensions:
  kubernetes:
    namespace: runfabric
    context: dev-cluster
    deploymentName: hello-api
    serviceName: hello-api
    ingressHost: api.dev.example.com
```

## State Backends

```yaml
state:
  backend: local # local|postgres|s3|gcs|azblob
  keyPrefix: runfabric/state
  lock:
    enabled: true
    timeoutSeconds: 30
    heartbeatSeconds: 10
    staleAfterSeconds: 60
```

Backend-specific options:

- `state.local.dir`
- `state.postgres.connectionStringEnv`, `state.postgres.schema`, `state.postgres.table`
- `state.s3.bucket`, `state.s3.region`, `state.s3.keyPrefix`, `state.s3.useLockfile`
- `state.gcs.bucket`, `state.gcs.prefix`
- `state.azblob.container`, `state.azblob.prefix`

**DB-backed deploy state (receipts):** Set `backend.kind` to `postgres`, `sqlite`, or `dynamodb` (and the corresponding `backend.*` options) to store and fetch deploy receipts from a database. See [STATE_BACKENDS.md](STATE_BACKENDS.md).

Detailed backend behavior: [STATE_BACKENDS.md](STATE_BACKENDS.md).

## Logs

Optional local log file source (unified with provider logs). When `logs.path` is set (or default `.runfabric/logs`), `runfabric logs` appends lines from:

- `<path>/<stage>.log` — stage-level log file
- `<path>/<function>_<stage>.log` — per-function log file (when requesting a single function)

Example:

```yaml
logs:
  path: .runfabric/logs   # default; directory relative to project root
```

Provider logs (e.g. CloudWatch for AWS) are fetched first; local file lines are appended to the same result.

## Build order

Optional ordering of build steps or hook modules. When you have multiple hooks (see [PLUGIN_API.md](PLUGIN_API.md)), `build.order` defines the execution order. Values can use `${env:VAR}`.

```yaml
build:
  order: ["deps", "compile", "bundle"]

hooks:
  - ./hooks/deps.mjs
  - ./hooks/compile.mjs
  - ./hooks/bundle.mjs
```

## Alerts

Optional alerting configuration. URLs support `${env:VAR}`. Delivery is integration-specific; the config is available for tooling or future runtime hooks.

```yaml
alerts:
  webhook: "${env:ALERT_WEBHOOK_URL}"
  slack: "${env:SLACK_WEBHOOK_URL}"
  onError: true
  onTimeout: true
```

- **`webhook`** — HTTP POST URL for alert payloads (errors, timeouts).
- **`slack`** — Slack webhook URL.
- **`onError`** / **`onTimeout`** — Enable triggers; used by integrations when emitting alerts.

## App and org

Optional grouping for dashboards or multi-service UIs:

```yaml
app: my-app
org: my-org
service: my-api
# ...
```

- **`app`** — Application or project group name.
- **`org`** — Organization or tenant identifier. Both support `${env:VAR}`.

## Add-ons

Add-ons are optional integrations (e.g. Sentry, Datadog) declared under `addons`. Each entry can specify:

- **`name`** (optional): Logical name; defaults to the map key.
- **`version`** (optional): Version or tag for the add-on.
- **`options`** (optional): Add-on-specific config (key/value).
- **`secrets`** (optional): Map of **env var name → ref**. At deploy, refs are resolved and the resulting values are injected into every function’s environment. A ref can be:
  - `${env:VAR}` — value from the process environment at deploy.
  - A key into the top-level **`secrets`** map, whose value is then resolved (e.g. `${env:VAR}`).

Example:

```yaml
secrets:
  sentry_dsn: "${env:SENTRY_DSN}"

addons:
  sentry:
    version: "1"
    options:
      tracesSampleRate: 1.0
    secrets:
      SENTRY_DSN: sentry_dsn   # uses secrets.sentry_dsn → ${env:SENTRY_DSN}
  datadog:
    secrets:
      DD_API_KEY: "${env:DD_API_KEY}"
```

Use `runfabric addons list` to see the built-in catalog; if `addonCatalogUrl` is set, the CLI fetches and merges entries from that URL. Validation ensures addon secret keys (env var names) are non-empty.

**Per-function addons:** In `functions.<name>` you can set **`addons`** to a list of addon keys (e.g. `["sentry"]`). Only those addons' secrets are injected into that function. If `addons` is omitted or empty, all top-level addons apply.

## Runtime fabric

When you want **active-active** deploy (same service in multiple regions or providers) with health checks and optional failover/latency routing, add a **`fabric`** block. It requires **`providerOverrides`**; each entry in `fabric.targets` is a provider key to deploy to.

- **`targets`** (required): List of provider keys (e.g. `["aws-us", "aws-eu"]`) to deploy to. Use `runfabric fabric deploy` to deploy to all targets and record endpoints in `.runfabric/fabric-<stage>.json`.
- **`healthCheck`** (optional): Same shape as `deploy.healthCheck`; used when running health checks on fabric endpoints.
- **`routing`** (optional): `failover`, `latency`, or `round-robin` — for documentation and future use; configure your DNS/load balancer (e.g. Route53) with the endpoints from `runfabric fabric endpoints`.

Example:

```yaml
providerOverrides:
  aws-us:
    name: aws-lambda
    runtime: nodejs
    region: us-east-1
  aws-eu:
    name: aws-lambda
    runtime: nodejs
    region: eu-west-1

fabric:
  targets: [aws-us, aws-eu]
  routing: latency
```

Then run `runfabric fabric deploy` (deploys to both), `runfabric fabric status` (HTTP GET each endpoint, report healthy/fail), and `runfabric fabric endpoints` (list URLs for use with Route53 or other DNS/LB).

## Managed resource binding

Declare database and cache resources so that `DATABASE_URL`, `REDIS_URL`, and similar connection strings are injected into every function’s environment at deploy. Values come from the process environment or from a literal/`${env:VAR}` expression.

Each entry under `resources` must have:

- **`envVar`** (required): The environment variable name to set (e.g. `DATABASE_URL`, `REDIS_URL`).
- **`connectionStringEnv`** or **`connectionString`** (one required):
  - **`connectionStringEnv`**: Name of an env var to read at deploy time (e.g. in CI set `DATABASE_URL` and reference it here).
  - **`connectionString`**: Literal value or `${env:VAR}` (and optional default) resolved at deploy.

**Optional provisioning (RDS, ElastiCache):** **`provision`** (boolean): when true, the engine calls the provider’s provision callback to obtain a connection string (e.g. RDS, ElastiCache). The config layer supports this via `ResourceProvisionFn`; if the provider does not implement it or returns an error, binding falls back to `connectionStringEnv` or `connectionString`. The **AWS provider** implements lookup for existing RDS and ElastiCache resources. Supported spec fields when `provision: true`:
- **RDS:** `type: "database"` or `"rds"`, **`identifier`** (DB instance ID), optional **`region`** (defaults to `AWS_REGION`), optional **`engine`** (`"postgres"` | `"mysql"`), and for building the URL: **`userEnv`**, **`passwordEnv`**, **`dbNameEnv`** (env var names for user, password, and database name). If `userEnv`/`passwordEnv` are not set or the env vars are empty, provisioning returns not-implemented and binding falls back to `connectionStringEnv`/`connectionString`.
- **ElastiCache:** `type: "cache"` or `"elasticache"`, **`identifier`** (replication group ID or cache cluster ID), optional **`region`**. Returns a `redis://host:port` connection string.

**Per-function resource refs:** In `functions.<name>` you can set **`resources`** to a list of resource keys (e.g. `["db"]`). Only those resources’ env vars are injected into that function. If `resources` is omitted or empty, all top-level resources are injected (current default).

Example:

```yaml
resources:
  db:
    type: database
    envVar: DATABASE_URL
    connectionStringEnv: DATABASE_URL   # value from process env at deploy
  cache:
    type: cache
    envVar: REDIS_URL
    connectionString: "${env:REDIS_URL}"  # or literal redis://localhost:6379
```

At deploy, each function’s environment is merged with these bindings (then with compose `SERVICE_*_URL` and other `extraEnv`). If a function sets `resources: [key1, ...]`, only those resources' env vars are injected; otherwise all resources apply. When `provision: true` is set, the engine calls the provider's Provisioner; if it returns not-implemented or error, the existing connectionStringEnv/connectionString path is used.

## Validation

See `engine/internal/config/validate.go`: provider name and runtime required (after normalize); at least one function; backend kind and S3 fields when applicable; event/authorizer rules.

## Schema files

| File | Purpose |
|------|---------|
| [schemas/runfabric.schema.json](../schemas/runfabric.schema.json) | Full schema (legacy + reference). |
| [schemas/resource.schema.json](../schemas/resource.schema.json) | Resource definition stub. |
| [schemas/workflow.schema.json](../schemas/workflow.schema.json) | Workflow definition stub. |
| [schemas/secrets.schema.json](../schemas/secrets.schema.json) | Secrets map shape. |

## Workflows And Secrets

```yaml
workflows:
  - name: order-flow
    steps:
      - function: create-order
        next: charge
      - function: charge
        retry:
          attempts: 3
          backoffSeconds: 5

secrets:
  DB_PASSWORD: secret://prod/db/password
```

## Related Docs

- [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md) — CLI commands and flags.
- [ROADMAP.md](ROADMAP.md) — Current scope and next steps.
- [QUICKSTART.md](QUICKSTART.md)
- [HANDLER_SCENARIOS.md](HANDLER_SCENARIOS.md)
