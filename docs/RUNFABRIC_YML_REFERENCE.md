# runfabric.yml Reference

Canonical config reference for the current release train. Aligned with [upstream RUNFABRIC_YML_REFERENCE](https://github.com/runfabric/runfabric/blob/main/docs/RUNFABRIC_YML_REFERENCE.md). RunFabric uses a canonical provider/function model (`provider`, optional `backend`/`state`, and `functions`). **JSON Schema:** [schemas/runfabric.schema.json](../schemas/runfabric.schema.json).

## Quick navigation

- **Start from a template**: Minimum Example
- **Find a key quickly**: Top-Level Fields
- **Multi-cloud**: providerOverrides
- **Safety**: Real deploy and unsafe defaults

## Minimum Example

```yaml
service: hello-api
provider:
  name: aws-lambda
  runtime: nodejs20.x
functions:
  - name: api
    entry: src/index.ts
    triggers:
      - type: http
        method: GET
        path: /hello
```

## Top-Level Fields

- **Core (required)**
  - `service` (`string`, required)
  - `provider` (`object`, required)
  - `functions` (`function[]`, required)

- **App definition**
  - `env` (`Record<string,string>`, optional)
  - `params` (`Record<string,string>`, optional)
  - `resources` (`object`, optional) — Managed resource binding: declare DB/cache and inject `DATABASE_URL`, `REDIS_URL`, etc. into function env at deploy. See [Managed resource binding](#managed-resource-binding).
  - `layers` (`Record<string, layer>`, optional) — First-class layer declarations. Key = logical name; value has `ref` (provider-specific identifier), plus optional `name`/`version`. See [First-class layers](#first-class-layers) below.

- **Extensions**
  - `addons` (`Record<string, addon>`, optional) — Add-on declarations (marketplace-style). See [Add-ons](#add-ons) and `runfabric extensions addons list`.
  - `addonCatalogUrl` (`string`, optional) — URL to fetch addon catalog entries (JSON array); merged with built-in when running `runfabric extensions addons list`.
  - `extensions` (`object`, optional) — Extension settings and provider-specific extension config.
  - `hooks` (`string[]`, optional)

- **Deploy, state, and environments**
  - `deploy` (`object`, optional)
  - `state` (`object`, optional)
  - `logs` (`object`, optional) — Local log file source for `runfabric invoke logs`. When set, logs are read from provider (e.g. CloudWatch) and merged with lines from local files. See [Logs](#logs).
  - `stages` (`Record<string, override>`, optional)
  - `providerOverrides` (`Record<string, provider>`, optional) — Multi-cloud: named provider configs. Use with `runfabric deploy --provider <key>`, `runfabric plan --provider <key>`, `runfabric remove --provider <key>`. Key is a logical name (e.g. `aws`, `gcp`); value is the same shape as `provider` (name, runtime, region, optional `source`/`version` for external plugin selection).
  - `fabric` (`object`, optional) — Runtime fabric for active-active deploy, health checks, and failover/latency routing. Requires `providerOverrides`. See [Runtime fabric](#runtime-fabric).

- **Org / UI**
  - `app` (`string`, optional) — Application or project group name for dashboard/UI grouping.
  - `org` (`string`, optional) — Organization or tenant identifier for multi-tenant dashboards.

- **Workflow and alerts**
  - `workflows` (`workflow[]`, optional)
  - `integrations` (`Record<string, object>`, optional) — Integration configuration blocks (including MCP-oriented settings and approval adapters).
  - `policies` (`Record<string, object>`, optional) — Policy configuration blocks applied by workflow/deploy/runtime policy engines.
  - `alerts` (`object`, optional) — Optional alerting config (webhook, Slack, triggers). See [Alerts](#alerts).
  - `build` (`object`, optional) — Build-step ordering. See [Build order](#build-order).
  - `secrets` (`Record<string,string>`, optional) — Secret map used by `${secret:KEY}` resolution. Values can be literals, `${env:VAR}` expressions, or `secret://OTHER_KEY` indirection.

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
    backend: # optional: per-provider state backend (when using --provider aws)
      kind: s3
      s3Bucket: my-aws-bucket
  gcp:
    name: gcp-functions
    runtime: nodejs
    region: us-central1
    source: external # optional: prefer external plugin over built-in
    version: 1.2.3 # optional: pin external plugin version
    backend: # optional: e.g. gcs for GCP
      kind: gcs

# ... functions, triggers, etc.
```

Then run e.g. `runfabric deploy --provider aws --stage prod` or `runfabric deploy --provider gcp --stage prod`. Without `--provider`, the top-level `provider` block is used. When a provider override includes `backend`, that backend is used for state (receipts, locks) when `--provider <key>` is set. Invoke, logs, metrics, and traces also accept `--provider` for multi-cloud.

## Auto-install missing extensions (plugins)

If your `provider.name` refers to an **external provider plugin** that is not installed on disk, lifecycle commands (plan/deploy/invoke/logs/etc.) will fail with “provider … not registered”.

To let RunFabric auto-install the missing provider from the registry, enable:

```yaml
extensions:
  autoInstallExtensions: true
```

You can force external provider resolution and pin a plugin version directly in provider config:

```yaml
provider:
  name: vercel
  runtime: nodejs
  source: external
  version: 1.2.3
```

Rules:

- `source` supports `builtin` (default) or `external`.
- `version` is valid only when `source: external`.
- `aws-lambda` remains internal while the plugin contract stabilizes; `source: external` is rejected for AWS Lambda.

Behavior:

- **Interactive (default)**: prompts `Install from registry? [y/N]` and continues on yes.
- **Non-interactive**: add `-y/--yes` to auto-accept, otherwise it fails safely.
- **Registry URL/token**: uses `RUNFABRIC_REGISTRY_URL` / `RUNFABRIC_REGISTRY_TOKEN` (or `.runfabricrc` `registry.url` / `registry.token` when present).

You can also ensure other plugin kinds are installed (best-effort) when auto-install is enabled:

```yaml
extensions:
  autoInstallExtensions: true
  runtimePlugin: nodejs # kind=runtime
  simulatorPlugin: local # kind=simulator
  routerPlugin: cloudflare # kind=router (router command backend)
  router:
    autoApply:
      enabled: true
      stages: [staging, prod]
      enforceStageRollout: true
    approvalEnvByStage:
      staging: RUNFABRIC_DNS_SYNC_DEV_APPROVED
      prod: RUNFABRIC_DNS_SYNC_STAGING_APPROVED
    requireReason: true
    reasonEnv: RUNFABRIC_DNS_SYNC_REASON
    mutationPolicy:
      enabled: true
      approvalEnv: RUNFABRIC_DNS_SYNC_RISK_APPROVED
      riskyResources: [lb_monitor, lb_pool, load_balancer]
      maxMutationsWithoutApproval: 3
    credentialPolicy:
      enabled: true
      requireAttestation: true
      attestationEnv: RUNFABRIC_ROUTER_TOKEN_ATTESTED
      issuedAtEnv: RUNFABRIC_ROUTER_TOKEN_ISSUED_AT
      expiresAtEnv: RUNFABRIC_ROUTER_TOKEN_EXPIRES_AT
      maxTTLSeconds: 3600
      minRemainingSeconds: 120
    credentials:
      zoneIDEnv: RUNFABRIC_ROUTER_ZONE_ID
      accountIDEnv: RUNFABRIC_ROUTER_ACCOUNT_ID
      apiTokenEnv: RUNFABRIC_ROUTER_API_TOKEN
      apiTokenFileEnv: RUNFABRIC_ROUTER_API_TOKEN_FILE
      apiTokenSecretRef: router_api_token
```

Router commands ship with built-in `cloudflare` backend. External `kind=router` plugins are also supported through extension discovery/dispatch. When omitted, `extensions.routerPlugin` defaults to `cloudflare`.
Additional built-ins: `route53`, `ns1`, `azure-traffic-manager` (provider API reconcilers).

Router policy keys under `extensions.router`:

- `autoApply` (`bool|object`): enable automatic post-deploy DNS sync from `runfabric router deploy`.
- `autoApply.stages` (`string[]`): restrict auto-apply to listed stages.
- `autoApply.dryRun` (`bool`): default dry-run for auto-applies.
- `allowProdSync` (`bool`): config default for prod sync gate.
- `enforceStageRollout` (`bool`): config default for staged rollout approval checks.
- `approvalEnvByStage` (`map`): stage -> env var that must equal `true`.
- `requireReason` + `reasonEnv`: require an operator reason env var before apply.
- `mutationPolicy`: policy-as-code guardrails for risky/high-volume mutations.
  - `enabled` (`bool`): turn on mutation preflight enforcement.
  - `approvalEnv` (`string`): env var that must equal `true` to approve guarded mutations.
  - `riskyResources` (`string[]`): resources requiring approval when changed.
  - `maxMutationsWithoutApproval` (`int`): mutation threshold requiring approval.
- `credentialPolicy`: policy-as-code guardrails for short-lived credentials.
  - `enabled` (`bool`): turn on credential attestation checks before sync.
  - `requireAttestation` + `attestationEnv`: require explicit approval signal (env must equal `true`).
  - `issuedAtEnv` / `expiresAtEnv`: RFC3339 timestamps used to validate token lifetime.
  - `maxTTLSeconds` (`int`): max allowed token lifetime.
  - `minRemainingSeconds` (`int`): minimum required remaining token lifetime at execution time.
- `credentials.zoneIDEnv` / `credentials.accountIDEnv`: customize env names for zone/account lookup.
- `credentials.apiTokenEnv` / `credentials.apiTokenFileEnv`: customize token env or file-env wiring (supports secret-manager mounted token files).
- `credentials.apiTokenSecretRef`: first-class secret reference (`KEY`, `${secret:KEY}`, or `secret://KEY`) resolved via top-level `secrets` + environment.
- `qualityScoring`: endpoint quality-based weight tuning.
  - `enabled` (`bool`): enable quality scoring before sync/simulate output is generated.
  - `unhealthyPenaltyPercent` (`int`): weight reduction for unhealthy endpoints (0-100).
  - `providerMultiplier` (`map[string]int`): provider-specific weight multipliers in percent (`100` = neutral).
- `canary`: default staged traffic shift policy.
  - `enabled` (`bool`): enable default canary weighting.
  - `provider` (`string`): endpoint/provider key to receive canary traffic.
  - `percent` (`int`): canary target percentage (1-99 recommended).
- `stages.<stage>`: stage-specific overrides for all router policy keys above.

## Real deploy and unsafe defaults

Real deploy is opt-in: set **`RUNFABRIC_REAL_DEPLOY=1`** or provider-specific **`RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1`** (e.g. `RUNFABRIC_CLOUDFLARE_REAL_DEPLOY=1`). When real deploy is enabled:

- **Credentials:** Provider-specific env vars (API keys, region, project ID) must be set; otherwise deploy will fail. Run `runfabric doctor` to check; it reports missing required env per provider (see [CREDENTIALS.md](CREDENTIALS.md), [PROVIDER_SETUP.md](PROVIDER_SETUP.md)).
- **Public HTTP:** Deploying HTTP endpoints without auth (no `authorizer` on the trigger) exposes the function to the internet. Prefer auth (e.g. `authorizer.type: jwt` or IAM) for production.
- **Secrets:** Use `secrets` and resource bindings rather than plain env for sensitive values.

## First-class layers

Define layers once and reference them by name from functions. Use `ref` for the provider-specific layer identifier:

```yaml
layers:
  node-deps:
    ref: "arn:aws:lambda:us-east-1:123456789012:layer:node-deps:1"
    name: node-deps
    version: "1"
  custom:
    ref: "${env:LAMBDA_LAYER_ARN}"
    version: "${env:LAYER_VERSION}" # optional: set from CI (e.g. package-lock hash)

functions:
  - name: api
    entry: src/handler.default
    layers: ["node-deps", "custom"]
```

Each function entry's `layers` list can use logical names (keys in top-level `layers`) or literal provider-specific layer refs. For AWS, ARNs continue to work.

**Versioning on dependency change:** Use `version` with an env var (e.g. `version: "${env:LAYER_VERSION}"`) and set that in CI from a hash of `package-lock.json` or `requirements.txt` so layer refs or versions track dependency changes. Resolve runs after env is set, so the same config works across environments.

**Other providers:** Layers are applied by AWS Lambda today. Other providers (GCP, Azure, etc.) preserve the `layers` config but do not apply it; use provider-specific mechanisms (e.g. build env, separate artifacts) where needed.

## Dynamic Env Bindings

String values can resolve environment variables using:

- `${env:VAR_NAME}`
- `${env:VAR_NAME,default-value}`

Example:

```yaml
service: ${env:RUNFABRIC_SERVICE_NAME,my-service}
provider:
  name: aws-lambda
  runtime: nodejs20.x
  region: ${env:AWS_REGION,us-east-1}
backend:
  kind: s3
  s3Bucket: ${env:RUNFABRIC_STATE_S3_BUCKET}
functions:
  - name: api
    entry: src/index.ts
    triggers:
      - type: http
        method: GET
        path: /hello
```

If `${env:VAR_NAME}` is used without a default and the variable is missing, config parsing fails with an explicit error.

## Secret References

String values can also resolve `${secret:KEY}` placeholders. Resolution order:

1. `secrets.KEY` from top-level config.
2. Environment variable `KEY`.

Top-level `secrets` entries support `secret://OTHER_KEY` indirection:

```yaml
secrets:
  db_url: secret://DATABASE_URL

functions:
  - name: api
    entry: src/handler.default
    env:
      DATABASE_URL: "${secret:db_url}"
```

If a `${secret:KEY}` reference cannot be resolved, config resolution fails with an explicit error.

## MCP Integrations and Policies

MCP configuration is provider-neutral and configured under `integrations.mcp` plus `policies.mcp`.

### Register MCP servers

```yaml
integrations:
  mcp:
    servers:
      crm:
        url: https://mcp.internal/crm
      kb:
        url: https://mcp.internal/kb
```

### Enforce MCP allow/deny policy

```yaml
policies:
  mcp:
    defaultDeny: true
    allow:
      servers: ["crm", "kb"]
      tools: ["crm.lookup*", "kb.search*"]
      resources: ["kb.kb://*"]
      prompts: ["crm.reply*"]
    deny:
      tools: ["crm.delete*"]
```

Policy semantics:

- `defaultDeny: true` blocks unmatched calls.
- `allow` and `deny` support wildcard patterns (`*`).
- `deny` wins over `allow` when both match.

### Provider-specific MCP policy rules

```yaml
policies:
  mcp:
    providers:
      aws-lambda:
        requiredRegion: us-east-1
        denyCrossRegion: true
        denyRegions: ["eu-*"]
        requiredAuth: iam
        models:
          default: anthropic.claude-3-sonnet-20240229-v1:0
          ai-eval: anthropic.claude-3-haiku-20240307-v1:0
```

Supported keys under `policies.mcp.providers.<provider>`:

- `requiredRegion`: required runtime region for MCP calls.
- `denyCrossRegion`: block calls when active region differs from `requiredRegion`.
- `denyRegions`: wildcard deny list for active regions.
- `requiredAuth`: required auth mode hint recorded in policy metadata.
- `models`: optional per-provider model override map for workflow AI steps.
- `models.default`: fallback model for all AI step kinds.
- `models.ai-retrieval|ai-generate|ai-structured|ai-eval`: per-kind model overrides.
- Per-step override: set `workflows[].steps[].model` (or `workflows[].steps[].input.model`) to force a model for one specific step.

Environment-based overrides are also supported:

- Global: `RUNFABRIC_MODEL_DEFAULT`, `RUNFABRIC_MODEL_AI_RETRIEVAL`, `RUNFABRIC_MODEL_AI_GENERATE`, `RUNFABRIC_MODEL_AI_STRUCTURED`, `RUNFABRIC_MODEL_AI_EVAL`
- Provider-scoped: `RUNFABRIC_MODEL_<PROVIDER>_DEFAULT` and `RUNFABRIC_MODEL_<PROVIDER>_AI_{RETRIEVAL|GENERATE|STRUCTURED|EVAL}` (example: `RUNFABRIC_MODEL_AWS_LAMBDA_AI_GENERATE`)
- Precedence: per-step `model` / `input.model` > `policies.mcp.providers.<provider>.models` > environment overrides > built-in provider fallback

## Deploy Policy

Single-function deploy: use `runfabric deploy --function <name>`, `runfabric deploy fn <name>`, `runfabric deploy function <name>`, or `runfabric deploy-function <name>`.

```yaml
deploy:
  rollbackOnFailure: true # optional
  strategy: all-at-once # optional: all-at-once (default), canary, blue-green
  canaryPercent: 10 # 0-100 when strategy: canary (provider-specific traffic shift)
  canaryIntervalMinutes: 5 # minutes before full shift when strategy: canary (optional)
  healthCheck: # optional post-deploy HTTP GET
    enabled: true
    url: "" # empty = use deployed URL from receipt (ServiceURL, url, ApiUrl)
  scaling: # optional provider-level defaults (overridden per function)
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
  - name: api
    entry: src/handler.default
    layers: ["node-deps"] # refs to top-level layers.* or literal provider-specific layer refs
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
3. Env toggle (`RUNFABRIC_ROLLBACK_ON_FAILURE`)

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
backend:
  kind: local # local|postgres|sqlite|s3|dynamodb|gcs|azblob
  s3Bucket: my-state-bucket # when kind=s3
  s3Prefix: runfabric/state # when kind=s3
  lockTable: runfabric-locks # when kind=s3 or kind=dynamodb
  gcsBucket: my-state-bucket # when kind=gcs
  gcsPrefix: runfabric/state # when kind=gcs
  azblobContainer: runfabric-state # when kind=azblob
  azblobPrefix: runfabric/state # when kind=azblob
  postgresConnectionStringEnv: RUNFABRIC_STATE_POSTGRES_URL # when kind=postgres
  postgresTable: runfabric_receipts # when kind=postgres
  sqlitePath: .runfabric/state.db # when kind=sqlite
  receiptTable: runfabric-receipts # when kind=dynamodb
```

Backend-specific options:

- `backend.s3Bucket`, `backend.s3Prefix`, `backend.lockTable`
- `backend.gcsBucket`, `backend.gcsPrefix`
- `backend.azblobContainer`, `backend.azblobPrefix`
- `backend.postgresConnectionStringEnv`, `backend.postgresTable`
- `backend.sqlitePath`
- `backend.receiptTable`

**DB-backed deploy state (receipts):** Set `backend.kind` to `postgres`, `sqlite`, or `dynamodb` (and the corresponding `backend.*` options) to store and fetch deploy receipts from a database. See [STATE_BACKENDS.md](STATE_BACKENDS.md).

Detailed backend behavior: [STATE_BACKENDS.md](STATE_BACKENDS.md).

## Logs

Optional local log file source (unified with provider logs). When `logs.path` is set (or default `.runfabric/logs`), `runfabric invoke logs` appends lines from:

- `<path>/<stage>.log` — stage-level log file
- `<path>/<function>_<stage>.log` — per-function log file (when requesting a single function)

Example:

```yaml
logs:
  path: .runfabric/logs # default; directory relative to project root
```

Provider logs (e.g. CloudWatch for AWS) are fetched first; local file lines are appended to the same result.

## Build order

Optional ordering of build steps or hook modules. When you have multiple hooks (see [PLUGINS.md](../apps/registry/docs/PLUGINS.md)), `build.order` defines the execution order. Values can use `${env:VAR}`.

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

## Add-ons (RunFabric Addons, Phase 15)

Add-ons are optional integrations (e.g. Sentry, Datadog) declared under `addons`. The **provider** and **runtime** fields elsewhere in config resolve to RunFabric Plugin IDs (e.g. `aws-lambda`, `nodejs`); use `runfabric extensions extension list` to see built-in plugins. Each entry can specify:

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
      SENTRY_DSN: sentry_dsn # uses secrets.sentry_dsn → ${env:SENTRY_DSN}
  datadog:
    secrets:
      DD_API_KEY: "${env:DD_API_KEY}"
```

Use `runfabric extensions addons list` to see the built-in catalog; if `addonCatalogUrl` is set, the CLI fetches and merges entries from that URL. Validation ensures addon secret keys (env var names) are non-empty.

**Per-function addons:** In each function entry under `functions`, set **`addons`** to a list of addon keys (e.g. `["sentry"]`). Only those addons' secrets are injected into that function. If `addons` is omitted or empty, all top-level addons apply.

## Runtime fabric

When you want **active-active** deploy (same service in multiple regions or providers) with health checks and optional failover/latency routing, add a **`fabric`** block. It requires **`providerOverrides`**; each entry in `fabric.targets` is a provider key to deploy to.

- **`targets`** (required): List of provider keys (e.g. `["aws-us", "aws-eu"]`) to deploy to. Use `runfabric router deploy` to deploy to all targets and record endpoints in `.runfabric/fabric-<stage>.json`.
- **`healthCheck`** (optional): Same shape as `deploy.healthCheck`; used when running health checks on fabric endpoints.
- **`routing`** (optional): `failover`, `latency`, or `round-robin` — for documentation and future use; configure your DNS/load balancer (e.g. Route53) with the endpoints from `runfabric router endpoints`.

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

Then run `runfabric router deploy` (deploys to both), `runfabric router status` (HTTP GET each endpoint, report healthy/fail), and `runfabric router endpoints` (list URLs for use with Route53 or other DNS/LB).

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

**Per-function resource refs:** In each function entry under `functions`, set **`resources`** to a list of resource keys (e.g. `["db"]`). Only those resources’ env vars are injected into that function. If `resources` is omitted or empty, all top-level resources are injected (current default).

Example:

```yaml
resources:
  db:
    type: database
    envVar: DATABASE_URL
    connectionStringEnv: DATABASE_URL # value from process env at deploy
  cache:
    type: cache
    envVar: REDIS_URL
    connectionString: "${env:REDIS_URL}" # or literal redis://localhost:6379
```

At deploy, each function’s environment is merged with these bindings (then with compose `SERVICE_*_URL` and other `extraEnv`). If a function sets `resources: [key1, ...]`, only those resources' env vars are injected; otherwise all resources apply. When `provision: true` is set, the engine calls the provider's Provisioner; if it returns not-implemented or error, the existing connectionStringEnv/connectionString path is used.

## Validation

See `platform/core/model/config/validate.go`: provider name/runtime required, at least one function, backend kind constraints, and event/authorizer rules.

## Integrations and policies

Use `integrations` and `policies` for workflow/runtime extension settings without changing core config fields.

Example (MCP + policy blocks):

```yaml
integrations:
  mcp:
    enabled: true
    server: runfabric-mcp
    transport: stdio
  approvals:
    provider: slack
    channel: ${env:APPROVALS_CHANNEL,#ops-approvals}

policies:
  workflow:
    maxRunSeconds: 1800
    denyModelFamilies: ["experimental"]
  deploy:
    requireRollbackOnFailure: true
```

Validation expectations:

- The schema validates `integrations` and `policies` as object maps.
- Each integration/policy key accepts nested object values (`additionalProperties: true`) so teams can extend behavior incrementally.
- Runtime-specific validation (for known integrations/policies) happens in command/runtime logic, not by rigid top-level schema enums.

## Workflow step kinds

Workflow steps support a typed `kind` for AI/human-in-the-loop flows:

- `code`
- `ai-retrieval`
- `ai-generate`
- `ai-structured`
- `ai-eval`
- `human-approval`

Minimal typed example:

```yaml
workflows:
  - name: release-flow
    steps:
      - id: gather-context
        kind: ai-retrieval
        input:
          query: "Summarize deploy risks for this commit"
          model: gpt-4.1
      - id: generate-plan
        kind: ai-structured
        input:
          prompt: "Create a release plan"
          schema:
            type: object
            properties:
              actions:
                type: array
                items: { type: string }
      - id: approve
        kind: human-approval
        input:
          approvalRequest: "Review generated release actions"
      - id: deploy
        kind: code
```

Step requirements:

- `id` is required and should be unique within a workflow.
- `kind` is required for every step.
- AI and approval kind-specific values are read from `steps[].input`.
- `steps[].model` is a shorthand for `steps[].input.model` (for AI step kinds).

## Human approval lifecycle

For `kind: human-approval`, workflow execution follows:

- `paused`: run awaits reviewer decision and persists state.
- `decision`: approval provider/user submits approve or reject with optional context.
- `resume`: run continues from the next configured step (or fails based on decision/policy).

Operational flow:

- Start run: `runfabric workflow run ...`
- Poll state: `runfabric workflow status --run-id <id> ...`
- Continue/replay after decision path updates: `runfabric workflow replay ...` (or cancel with `workflow cancel`)

Approval inputs typically include the `approval.inputKey` payload from prior steps, reviewer identity, and optional justification captured by the integration.

## Provider-native orchestration extensions

Provider orchestration adapters are configured under `extensions`.

### GCP Cloud Workflows

Use `extensions.gcp-functions.cloudWorkflows` for workflow sync, invoke, and inspect:

```yaml
extensions:
  gcp-functions:
    cloudWorkflows:
      - name: order-flow
        definitionPath: workflows/order-flow.yaml
        bindings:
          createOrder: createOrder
```

Supported fields per item:

- `name` (required)
- one of `definition` (inline object) or `definitionPath` (path from project root)
- optional `bindings` map. Tokens `${bindings.key}` / `{{bindings.key}}` in workflow definitions are replaced with function resource identifiers from deploy context.

### Azure Durable Functions

Use `extensions.azure-functions.durableFunctions` for durable orchestration routing:

```yaml
extensions:
  azure-functions:
    durableFunctions:
      - name: order-flow
        orchestrator: OrderFlowOrchestrator
        taskHub: order-hub
        storageConnectionSetting: AzureWebJobsStorage
```

Supported fields per item:

- `name` (required)
- `orchestrator` (optional; defaults to `name`)
- `taskHub` (optional)
- `storageConnectionSetting` (optional)

Durable declarations are now applied through explicit Azure management-plane app settings updates during orchestration sync/remove. RunFabric writes and removes managed keys under `RUNFABRIC_DURABLE_<NAME>_*` so durable lifecycle state is explicit and reversible.

## Schema files

| File                                                              | Purpose                                                              |
| ----------------------------------------------------------------- | -------------------------------------------------------------------- |
| [schemas/runfabric.schema.json](../schemas/runfabric.schema.json) | Full schema for the current config contract.                         |
| [schemas/resource.schema.json](../schemas/resource.schema.json)   | Resource definition schema (binding + optional provisioning fields). |
| [schemas/workflow.schema.json](../schemas/workflow.schema.json)   | Workflow definition schema (`name`, `steps`, kind/input/model/timeout/retry shape). |
| [schemas/secrets.schema.json](../schemas/secrets.schema.json)     | Secrets map shape.                                                   |

## Related Docs

- [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md) — CLI commands and flags.
- [MCP.md](MCP.md) — MCP server (plan, deploy, doctor, invoke) for agents and IDEs.
- [QUICKSTART.md](QUICKSTART.md)
