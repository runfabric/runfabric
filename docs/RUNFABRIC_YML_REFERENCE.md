# runfabric.yml Reference

Canonical config reference for the current release train.

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
- `resources` (`object`, optional)
- `secrets` (`Record<string,string>`, optional; value format `secret://<ref>`)
- `workflows` (`workflow[]`, optional)
- `params` (`Record<string,string>`, optional)
- `extensions` (`object`, optional)
- `state` (`object`, optional)
- `stages` (`Record<string, override>`, optional)

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
    iam:
      role:
        statements:
          - effect: Allow
            actions:
              - s3:GetObject
            resources:
              - arn:aws:s3:::uploads/*
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

Detailed backend behavior: `docs/STATE_BACKENDS.md`.

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

- `docs/RUNFABRIC_YML_SCHEMA_PROPOSAL.md`
- `docs/QUICKSTART.md`
- `docs/HANDLER_SCENARIOS.md`
