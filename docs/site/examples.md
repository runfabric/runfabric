# Examples

## Scaffold Command Index

These are exact `init` command patterns used to create common starters.

HTTP:

```bash
runfabric init --dir ./runfabric-aws-lambda-http-state-local --template api --provider aws-lambda --state-backend local --lang ts --skip-install --no-interactive
```

Queue:

```bash
runfabric init --dir ./runfabric-aws-lambda-queue-state-local --template queue --provider aws-lambda --state-backend local --lang ts --skip-install --no-interactive
```

Storage (start with `worker`, then replace trigger block):

```bash
runfabric init --dir ./runfabric-aws-lambda-storage-state-local --template worker --provider aws-lambda --state-backend local --lang ts --skip-install --no-interactive
```

```yaml
triggers:
  - type: storage
    bucket: uploads
    events:
      - s3:ObjectCreated:*
```

EventBridge (start with `worker`, then replace trigger block):

```bash
runfabric init --dir ./runfabric-aws-lambda-eventbridge-state-local --template worker --provider aws-lambda --state-backend local --lang ts --skip-install --no-interactive
```

```yaml
triggers:
  - type: eventbridge
    pattern:
      source:
        - app.source
    bus: default
```

PubSub (GCP, start with `worker`, then replace trigger block):

```bash
runfabric init --dir ./runfabric-gcp-functions-pubsub-state-local --template worker --provider gcp-functions --state-backend local --lang ts --skip-install --no-interactive
```

```yaml
triggers:
  - type: pubsub
    topic: projects/my-project/topics/events
```

## Single Service

- `examples/hello-http/runfabric.quickstart.yml`
- Provider-specific variants in `examples/hello-http/`

## Handler Scenarios

- `examples/handler-scenarios/single-handler/`
- `examples/handler-scenarios/multi-handler/`
- `examples/handler-scenarios/README.md`
- Details and wrapper patterns: `docs/HANDLER_SCENARIOS.md`

## Compose Contracts

- `examples/compose-contracts/runfabric.compose.yml`
- Cross-service output contract variables are documented in `examples/compose-contracts/README.md`

## Capability Matrix

- Provider x trigger matrix: `docs/EXAMPLES_MATRIX.md`
- Validation checklist for scaffolded examples: `docs/EXAMPLE_VALIDATION.md`
