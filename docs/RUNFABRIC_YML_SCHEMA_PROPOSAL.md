# runfabric.yml Schema Proposal (Current)

Status: implemented schema and planner behavior.

For day-to-day usage use `docs/RUNFABRIC_YML_REFERENCE.md`. This file tracks schema design intent/details.

## Goal

Define exact `runfabric.yml` fields for:

- trigger variants across providers
- workflows
- resources
- secrets
- AWS extension fields
- state backend configuration + locking controls

## Top-Level Fields

- `service`: string (required)
- `runtime`: string (required)
- `entry`: string (required)
- `providers`: string[] (required, min 1)
- `triggers`: trigger[] (required, min 1)
- `functions`: function[] (optional)
- `hooks`: string[] (optional)
- `resources`: resources object (optional)
- `env`: `Record<string, string>` (optional)
- `secrets`: `Record<string, string>` (optional, values must be `secret://<ref>`)
- `workflows`: workflow[] (optional)
- `params`: `Record<string, string>` (optional)
- `extensions`: provider extensions object (optional)
- `state`: state backend config (optional)
- `stages`: stage override map (optional)

## Trigger Union (`triggers[]`)

### HTTP

- `type: http`
- `method`: string
- `path`: string

### Cron

- `type: cron`
- `schedule`: string
- `timezone?`: string

### Queue

- `type: queue`
- `queue`: string
- `batchSize?`: number
- `maximumBatchingWindowSeconds?`: number
- `maximumConcurrency?`: number
- `enabled?`: boolean
- `functionResponseType?`: `ReportBatchItemFailures`

### Storage

- `type: storage`
- `bucket`: string
- `events`: string[] (min 1)
- `prefix?`: string
- `suffix?`: string
- `existingBucket?`: boolean

### EventBridge

- `type: eventbridge`
- `pattern`: object
- `bus?`: string

### Pub/Sub

- `type: pubsub`
- `topic`: string
- `subscription?`: string

### Kafka

- `type: kafka`
- `brokers`: string[] (min 1)
- `topic`: string
- `groupId`: string

### RabbitMQ

- `type: rabbitmq`
- `queue`: string
- `exchange?`: string
- `routingKey?`: string

## Functions (`functions[]`)

- `name`: string (required)
- `entry?`: string
- `runtime?`: string
- `triggers?`: trigger[]
- `resources?`: resources object
- `env?`: `Record<string, string>`

## Resources (`resources`)

- `memory?`: number
- `timeout?`: number
- `queues?`: `{ name: string, ... }[]`
- `buckets?`: `{ name: string, ... }[]`
- `topics?`: `{ name: string, ... }[]`
- `databases?`: `{ name: string, ... }[]`

## Workflows (`workflows[]`)

- `name`: string (required)
- `steps`: workflow step[] (required, min 1)

Workflow step:

- `function`: string (required)
- `next?`: string
- `retry?`: object
- `retry.attempts?`: number (>= 1)
- `retry.backoffSeconds?`: number (>= 0)
- `timeoutSeconds?`: number (>= 1)

## Secrets (`secrets`)

- key: env-style secret key
- value: `secret://<ref>`

Deploy contract:

- provider adapters materialize provider references from `secret://...`
- plaintext secret values are not written to runfabric state

## State Block

- `state.backend`: `local | postgres | s3 | gcs | azblob` (default `local`)
- `state.keyPrefix?`: string (default `runfabric/state`)
- `state.lock.enabled?`: boolean
- `state.lock.timeoutSeconds?`: number
- `state.lock.heartbeatSeconds?`: number
- `state.lock.staleAfterSeconds?`: number
- `state.local.dir?`: string
- `state.postgres.connectionStringEnv?`: string
- `state.postgres.schema?`: string
- `state.postgres.table?`: string
- `state.s3.bucket`: string (required when backend is `s3`)
- `state.s3.region?`: string
- `state.s3.keyPrefix?`: string
- `state.s3.useLockfile?`: boolean
- `state.gcs.bucket`: string (required when backend is `gcs`)
- `state.gcs.prefix?`: string
- `state.azblob.container`: string (required when backend is `azblob`)
- `state.azblob.prefix?`: string

## Extensions (`extensions`)

Typed provider extension fields currently validated:

- `extensions.aws-lambda.stage`: string
- `extensions.aws-lambda.region`: string
- `extensions.aws-lambda.iam`: object
- `extensions.gcp-functions.region`: string
- `extensions.azure-functions.functionApp`: string
- `extensions.azure-functions.routePrefix`: string
- `extensions.cloudflare-workers.scriptName`: string
- `extensions.vercel.projectName`: string
- `extensions.netlify.siteName`: string
- `extensions.alibaba-fc.region`: string
- `extensions.digitalocean-functions.namespace`: string
- `extensions.digitalocean-functions.region`: string
- `extensions.fly-machines.appName`: string
- `extensions.fly-machines.region`: string
- `extensions.ibm-openwhisk.namespace`: string

AWS IAM schema:

- `extensions.aws-lambda.iam.role.statements[]`
- `sid?`: string
- `effect`: `Allow | Deny`
- `actions`: string[]
- `resources`: string[]
- `condition?`: object

## Deploy/State Contract for Resources, Workflows, Secrets

Provider deploys can return:

- `resourceAddresses`: `Record<string, string>`
- `workflowAddresses`: `Record<string, string>`
- `secretReferences`: `Record<string, string>`

State stores these under provider state record fields:

- `resourceAddresses`
- `workflowAddresses`
- `secretReferences`

Example local state file path:

- `.runfabric/state/<service>/<stage>/<provider>.state.json`
