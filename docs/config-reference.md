# runfabric.yml config reference

This document summarizes the main keys supported in `runfabric.yml`. For full behavior, see the config loader and validation in `internal/config`.

## Top-level keys

| Key         | Required | Description |
|------------|----------|-------------|
| `service`  | Yes      | Service name. |
| `provider` | Yes      | Provider block: `name`, `runtime`, `region`. |
| `backend`  | No       | State backend: `kind` (local, s3, gcs, azblob, postgres); for `s3`: `s3Bucket`, `s3Prefix`, `lockTable`. |
| `functions`| Yes      | Map of function names to function config. |
| `stages`   | No       | Stage-specific overrides. |

## Provider

- `name`: e.g. aws-lambda, gcp-functions, vercel (see capability matrix).
- `runtime`: e.g. nodejs20.x, python3.11.
- `region`: optional; default from env (e.g. AWS_REGION).

## Backend

- `kind`: `local` (default), `s3`, `gcs`, `azblob`, `postgres`. For `s3`, also set `s3Bucket` and `lockTable` (DynamoDB table for locking).

## Functions

Each function supports: `handler`, `runtime`, `memory`, `timeout`, `architecture` (x86_64, arm64), `environment`, `tags`, `layers`, `secrets`, `events`.

### Events

- `http`: `path`, `method`, optional `cors`, `authorizer`, `routeSettings`.
- `cron`: string (e.g. rate expression).
- `queue`: `queue` name.
- `storage`: `bucket`, optional `prefix`, `suffix`, `events` (e.g. s3:ObjectCreated:*).
- `eventbridge`: `pattern`, optional `bus`.
- `pubsub`: `topic`, optional `subscription`.

Validation rules (see `internal/config/validate.go`): queue trigger requires queue name; storage requires bucket; pubsub requires topic; HTTP authorizer types: jwt, lambda, iam.
