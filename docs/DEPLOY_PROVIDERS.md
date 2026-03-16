# Deploy by Provider

How deployment works per cloud provider. The Go engine uses REST/SDK for deploy where implemented; otherwise it may run in simulated or stub mode.

- **Credentials and wiring:** [PROVIDER_SETUP.md](PROVIDER_SETUP.md)
- **Credentials matrix:** [CREDENTIALS.md](CREDENTIALS.md)
- **Trigger support per provider:** [EXAMPLES_MATRIX.md](EXAMPLES_MATRIX.md)

## Deploy flow

1. `runfabric doctor` — validate config and credentials.
2. `runfabric plan` — show planned changes.
3. `runfabric build` — produce artifacts.
4. `runfabric deploy` — deploy to the configured provider.

Real deploy is opt-in: set **`RUNFABRIC_REAL_DEPLOY=1`** or provider-specific **`RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1`**.

## Provider-specific notes

- **aws-lambda:** Uses Lambda, API Gateway, SQS, S3, EventBridge as per [EXAMPLES_MATRIX.md](EXAMPLES_MATRIX.md). State backend can use S3 + DynamoDB (see [STATE_BACKENDS.md](STATE_BACKENDS.md)).
- **gcp-functions:** Cloud Functions (Gen2) and Pub/Sub where supported.
- **azure-functions:** Azure Functions; storage and triggers per matrix.
- **Others:** See [PROVIDER_SETUP.md](PROVIDER_SETUP.md) for credentials and [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md) for deploy flags.

**See also:** [BUILD_AND_RELEASE.md](BUILD_AND_RELEASE.md), [ARCHITECTURE.md](ARCHITECTURE.md).
