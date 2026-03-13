# Examples Matrix

Matrix of current official examples and provider trigger capabilities.

Runtime status:

- Runtime families are provider-capability based: `nodejs|python|go|java|rust|dotnet`

## Provider Example Configs

`examples/hello-http/` includes provider-specific configs:

- `runfabric.aws-lambda.yml`
- `runfabric.gcp-functions.yml`
- `runfabric.azure-functions.yml`
- `runfabric.kubernetes.yml`
- `runfabric.cloudflare-workers.yml`
- `runfabric.vercel.yml`
- `runfabric.netlify.yml`
- `runfabric.alibaba-fc.yml`
- `runfabric.digitalocean-functions.yml`
- `runfabric.fly-machines.yml`
- `runfabric.ibm-openwhisk.yml`

## Trigger Capability Matrix

Legend: `Y` supported, `N` not supported by planner capability matrix.

| Provider | http | cron | queue | storage | eventbridge | pubsub | kafka | rabbitmq |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| aws-lambda | Y | Y | Y | Y | Y | N | N | N |
| gcp-functions | Y | Y | Y | Y | N | Y | N | N |
| azure-functions | Y | Y | Y | Y | N | N | N | N |
| kubernetes | Y | Y | N | N | N | N | N | N |
| cloudflare-workers | Y | Y | N | N | N | N | N | N |
| vercel | Y | Y | N | N | N | N | N | N |
| netlify | Y | Y | N | N | N | N | N | N |
| alibaba-fc | Y | Y | Y | Y | N | N | N | N |
| digitalocean-functions | Y | Y | N | N | N | N | N | N |
| fly-machines | Y | N | N | N | N | N | N | N |
| ibm-openwhisk | Y | Y | N | N | N | N | N | N |

Source of truth: `packages/planner/src/capability-matrix.ts`

## Scenario Examples

- Handler wrappers and multi-handler: `examples/handler-scenarios/README.md`
- Compose multi-service contracts: `examples/compose-contracts/README.md`

## Migration Example

Use migration command to bootstrap from `serverless.yml`:

```bash
runfabric migrate --input ./serverless.yml --output ./runfabric.yml --json
```
