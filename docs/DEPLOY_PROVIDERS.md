# Deploy by Provider

How deployment works per cloud provider. The Go engine resolves providers through the extension boundary, then dispatches to control-plane, API, or provider-plugin execution paths.

- **Credentials and wiring:** [PROVIDER_SETUP.md](PROVIDER_SETUP.md)
- **Credentials matrix:** [CREDENTIALS.md](CREDENTIALS.md)
- **Trigger support per provider:** [EXAMPLES_MATRIX.md](EXAMPLES_MATRIX.md)

## Quick navigation

- **High-level flow**: Deploy flow
- **Which providers use API dispatch**: Providers wired to deploy API
- **Per-provider notes**: Provider-specific notes

## Deploy flow

1. `runfabric doctor` — validate config and credentials.
2. `runfabric plan` — show planned changes.
3. `runfabric build` — produce artifacts.
4. `runfabric deploy` — deploy to the configured provider.

Real deploy is opt-in: set **`RUNFABRIC_REAL_DEPLOY=1`** or provider-specific **`RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1`**.

If a provider is not registered (built-in or installed external plugin), commands fail with a provider registration error instead of a stub fallback.

## Providers wired to deploy API

The following providers have API-based deploy/remove/invoke/logs (see `engine/internal/deploy/api`). The list is asserted by tests so this doc stays in sync with code:

- **alibaba-fc**, **azure-functions**, **cloudflare-workers**, **digitalocean-functions**, **fly-machines**, **gcp-functions**, **ibm-openwhisk**, **kubernetes**, **netlify**, **vercel**

(AWS uses the control-plane path and is not in the API runner map.)

## Provider-specific notes

- **aws-lambda:** Uses Lambda, API Gateway, SQS, S3, EventBridge as per [EXAMPLES_MATRIX.md](EXAMPLES_MATRIX.md). State backend can use S3 + DynamoDB (see [STATE_BACKENDS.md](STATE_BACKENDS.md)).
- **gcp-functions:** Cloud Functions (Gen2) and Pub/Sub where supported.
- **azure-functions:** Azure Functions; storage and triggers per matrix.
- **Others:** See [PROVIDER_SETUP.md](PROVIDER_SETUP.md) for credentials and [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md) for deploy flags.

**See also:** [BUILD_AND_RELEASE.md](BUILD_AND_RELEASE.md), [ARCHITECTURE.md](ARCHITECTURE.md).
