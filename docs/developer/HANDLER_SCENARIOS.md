# Handler Scenarios

Use this guide when you need more than a single default `handler`. Aligned with [upstream HANDLER_SCENARIOS](https://github.com/runfabric/runfabric/blob/main/docs/HANDLER_SCENARIOS.md). In this repo the Node runtime adapter is provided by `@runfabric/sdk`; use `createHandler` from `@runfabric/sdk` or its adapters subpath.

## Quick navigation

- **One handler**: Scenario 1
- **Multiple functions**: Scenario 2
- **Framework apps (Express/Fastify/Nest)**: Scenario 3
- **Non-HTTP triggers**: Scenario 4
- **Examples**: Related Examples

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
import type { UniversalHandler } from "@runfabric/sdk";

export const handler: UniversalHandler = async (req) => ({
  status: 200,
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ method: req.method, path: req.path }),
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

Use `createHandler` from `@runfabric/sdk`:

```ts
import type { UniversalHandler } from "@runfabric/sdk";
import { createHandler } from "@runfabric/sdk";

export const handler: UniversalHandler = createHandler(appOrFastifyOrNestApp);
```

Framework wiring checklist:

- Install runtime adapter: `npm i @runfabric/sdk`
- Express dependency: `npm i express`
- Fastify dependency: `npm i fastify`
- Nest dependencies: `npm i @nestjs/core @nestjs/common @nestjs/platform-express reflect-metadata rxjs`
- TypeScript projects should include framework typings as needed (`@types/express`, etc.).
- Nest TypeScript config must set `"experimentalDecorators": true`.
- Nest TypeScript config must set `"emitDecoratorMetadata": true`.
- Ensure `reflect-metadata` is imported before Nest bootstrap where required.

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
