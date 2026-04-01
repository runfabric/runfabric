# gcp-secret-manager extension

External `kind=secret-manager` plugin that resolves `gcp-sm://...` references through Google Cloud Secret Manager.

This plugin shells out to `gcloud`, so the environment must have authenticated `gcloud` access.

## Build

```bash
go build -o bin/gcp-secret-manager ./extensions/secretmanagers/gcp
```

## Install (local)

Copy this folder to:

```text
$RUNFABRIC_HOME/plugins/secret-managers/gcp-secret-manager/0.1.0/
```

## Configure

```yaml
extensions:
  secretManagerPlugin: gcp-secret-manager

secrets:
  db_password: gcp-sm://my-project/prod-db-password/latest
```

## Reference format

- Scheme: `gcp-sm://`
- Supported forms:
  - `gcp-sm://<project>/<secret>`
  - `gcp-sm://<project>/<secret>/<version>`
  - `gcp-sm://<secret>?project=<project>&version=<version>`
- Query params (optional):
  - `project`
  - `version` (defaults to `latest`)
  - `jsonKey` (extract from JSON secret payload)

If project is omitted, the plugin falls back to `GCP_PROJECT_ID` then `GOOGLE_CLOUD_PROJECT`.
