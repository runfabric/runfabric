# aws-secret-manager extension

External `kind=secret-manager` plugin that resolves `aws-sm://...` references from AWS Secrets Manager.

## Build

```bash
go build -o bin/aws-secret-manager ./extensions/secretmanagers/aws
```

## Install (local)

Copy this folder to:

```text
$RUNFABRIC_HOME/plugins/secret-managers/aws-secret-manager/0.1.0/
```

## Configure

```yaml
extensions:
  secretManagerPlugin: aws-secret-manager

secrets:
  db_password: aws-sm://prod/my-service/db-password?region=us-east-1
```

## Reference format

- Scheme: `aws-sm://`
- Base: `aws-sm://<secret-id>`
- Query params (optional):
  - `region` (defaults to `AWS_REGION` or `AWS_DEFAULT_REGION`)
  - `versionStage`
  - `versionId`
  - `jsonKey` (extract a string field from JSON `SecretString`)

Example:

```text
aws-sm://prod/my-service/api-keys?region=us-east-1&jsonKey=public
```
