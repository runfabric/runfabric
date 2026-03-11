# Handler Scenarios

Use this guide when you need more than a single default `handler`.

## Scenario 1: Single Handler

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

```ts
import type { UniversalHandler } from "@runfabric/core";

export const handler: UniversalHandler = async (req) => ({
  status: 200,
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ method: req.method, path: req.path })
});
```

## Scenario 2: Multiple Named Handlers

```yaml
service: multi-api
runtime: nodejs
entry: src/index.ts

providers:
  - aws-lambda

triggers:
  - type: http
    method: GET
    path: /health

functions:
  - name: public-api
    entry: src/public.ts
    triggers:
      - type: http
        method: GET
        path: /public
  - name: admin-api
    entry: src/admin.ts
    triggers:
      - type: http
        method: POST
        path: /admin
```

Deploy one function quickly:

```bash
runfabric deploy --function public-api -c runfabric.yml
```

## Scenario 3: Existing Framework Apps

Use `createHandler` from `@runfabric/runtime-node`:

```ts
import type { UniversalHandler } from "@runfabric/core";
import { createHandler } from "@runfabric/runtime-node";

export const handler: UniversalHandler = createHandler(appOrFastifyOrNestApp);
```

## Scenario 4: Queue + Storage

```yaml
triggers:
  - type: queue
    queue: arn:aws:sqs:us-east-1:123456789012:jobs
  - type: storage
    bucket: uploads
    events:
      - s3:ObjectCreated:*
```

## Related Examples

- `examples/handler-scenarios/README.md`
- `examples/handler-scenarios/single-handler/runfabric.yml`
- `examples/handler-scenarios/multi-handler/runfabric.yml`
