# Sentry addon (local scaffold)

Local Node addon implementation for RunFabric.

## What this scaffold does

- Exposes the Addon contract fields: name, kind, version, supports, apply.
- Supports `runtime: nodejs` for `provider: aws` and `aws-lambda`.
- Reads addon options and maps them to env hints:
  - `tracesSampleRate` -> `SENTRY_TRACES_SAMPLE_RATE`
  - `environment` -> `SENTRY_ENVIRONMENT`
  - `release` -> `SENTRY_RELEASE`
- Generates helper file:
  - `.runfabric/generated/sentry/init-sentry.cjs`

This scaffold does not auto-patch handlers. It generates a helper and emits warnings so app code can opt in explicitly.

## Example config

```yaml
secrets:
  sentry_dsn: "${env:SENTRY_DSN}"

addons:
  sentry:
    version: "0.1.0"
    options:
      tracesSampleRate: 1.0
      environment: "prod"
    secrets:
      SENTRY_DSN: sentry_dsn
```

## Example app usage

```js
const Sentry = require("@sentry/node");
const { initSentry } = require("./.runfabric/generated/sentry/init-sentry.cjs");

initSentry(Sentry);
```
