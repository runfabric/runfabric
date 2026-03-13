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
- `deploy` (`object`, optional)
- `state` (`object`, optional)
- `stages` (`Record<string, override>`, optional)

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

```yaml
deploy:
  rollbackOnFailure: true # optional
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

- `docs/QUICKSTART.md`
- `docs/HANDLER_SCENARIOS.md`
