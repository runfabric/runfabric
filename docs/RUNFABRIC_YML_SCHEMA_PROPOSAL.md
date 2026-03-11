# runfabric.yml Schema Proposal (AWS Event + IAM Parity)

Status: proposed schema for next implementation phase.

## Goal

Define exact `runfabric.yml` fields for:

- SQS event source mappings
- S3 object event triggers
- AWS IAM role statements
- function-level environment variables

## Proposed Field Paths

### `triggers[]` union

`triggers[].type` stays required and supports these exact variants.

#### HTTP Trigger

- `triggers[].type`: `"http"` (required)
- `triggers[].method`: string (required)
- `triggers[].path`: string (required)

#### Cron Trigger

- `triggers[].type`: `"cron"` (required)
- `triggers[].schedule`: string (required)
- `triggers[].timezone`: string (optional)

#### Queue Trigger (SQS)

- `triggers[].type`: `"queue"` (required)
- `triggers[].queue`: string (required, queue name/URL/ARN)
- `triggers[].batchSize`: number (optional)
- `triggers[].maximumBatchingWindowSeconds`: number (optional)
- `triggers[].maximumConcurrency`: number (optional)
- `triggers[].enabled`: boolean (optional, default `true`)
- `triggers[].functionResponseType`: string (optional, allowed: `ReportBatchItemFailures`)

#### Storage Trigger (S3)

- `triggers[].type`: `"storage"` (required)
- `triggers[].bucket`: string (required)
- `triggers[].events`: string[] (required, at least 1)
- `triggers[].prefix`: string (optional)
- `triggers[].suffix`: string (optional)
- `triggers[].existingBucket`: boolean (optional, default `true`)

### Function-level env

- `functions[].env`: `Record<string, string>` (optional)

Merge order during deploy:

1. `env` (root)
2. `functions[].env` (function override wins)

### AWS IAM extension

- `extensions.aws-lambda.iam.role.statements[]`: array (optional)
- `extensions.aws-lambda.iam.role.statements[].sid`: string (optional)
- `extensions.aws-lambda.iam.role.statements[].effect`: `"Allow" | "Deny"` (required)
- `extensions.aws-lambda.iam.role.statements[].actions`: string[] (required, at least 1)
- `extensions.aws-lambda.iam.role.statements[].resources`: string[] (required, at least 1)
- `extensions.aws-lambda.iam.role.statements[].condition`: object (optional)

## Example: SQS Worker

```yaml
service: queue-worker
runtime: nodejs
entry: src/worker.ts

providers:
  - aws-lambda

triggers:
  - type: queue
    queue: arn:aws:sqs:us-west-1:123456789012:jobs
    batchSize: 10
    maximumBatchingWindowSeconds: 5
    maximumConcurrency: 2
    functionResponseType: ReportBatchItemFailures
```

## Example: S3 Event + IAM + Function Env

```yaml
service: fetch-file-and-store-in-s3
runtime: nodejs
entry: src/handler.ts

providers:
  - aws-lambda

triggers:
  - type: storage
    bucket: my-upload-bucket
    events:
      - s3:ObjectCreated:*
    prefix: incoming/
    suffix: .json

extensions:
  aws-lambda:
    region: us-west-1
    iam:
      role:
        statements:
          - effect: Allow
            actions:
              - s3:PutObject
              - s3:PutObjectAcl
            resources:
              - arn:aws:s3:::my-upload-bucket/*

functions:
  - name: save
    entry: src/handler.ts
    env:
      BUCKET: my-upload-bucket
```

## Backward Compatibility Rules

- Existing `queue` trigger config (`type: queue` + `queue`) remains valid.
- `http` and `cron` trigger shapes remain unchanged.
- Existing scalar-only `extensions.aws-lambda.stage` and `extensions.aws-lambda.region` remain valid.
