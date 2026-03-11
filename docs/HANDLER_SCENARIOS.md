# Handler Scenarios

Use this guide when you need more than a single default `handler`.

## Scenario 1: Single Handler

Use one entry file and one handler export.

Config:

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

Handler:

```ts
import type { UniversalHandler } from "@runfabric/core";

export const handler: UniversalHandler = async (req) => ({
  status: 200,
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ method: req.method, path: req.path })
});
```

## Scenario 2: Multiple Named Handlers

Use `functions` when you need separate handlers, entries, and trigger sets.

Config:

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

Deploy only one function:

```bash
runfabric deploy function public-api -c runfabric.yml
```

Local-call a specific function entry:

```bash
runfabric call-local -c runfabric.yml --entry src/public.ts --method GET --path /public
```

## Scenario 3: Express/Fastify/NestJS Wrappers

Use one wrapper API:

```ts
import type { UniversalHandler } from "@runfabric/core";
import { createHandler } from "@runfabric/runtime-node";

export const handler: UniversalHandler = createHandler(appOrFastifyOrNestApp);
```

Express catch-all example:

```ts
import express from "express";
import { createHandler } from "@runfabric/runtime-node";

const app = express();
app.use(express.json());
app.all("*", (req, res) => {
  res.status(200).json({ method: req.method, path: req.originalUrl });
});

export const handler = createHandler(app);
```

## Ready-Made Example Files

- `examples/handler-scenarios/single-handler/`
- `examples/handler-scenarios/multi-handler/`
- `examples/handler-scenarios/README.md`
