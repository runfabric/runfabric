# Handler Scenarios

This folder contains examples for common handler patterns:

- `single-handler/`: one `handler` for one entrypoint
- `multi-handler/`: multiple named functions with different entries and triggers
- framework wrappers (Express/Fastify/NestJS) using `createHandler`

## 1) Single handler

Files:

- `single-handler/runfabric.yml`
- `single-handler/src/index.ts`

Run locally:

```bash
pnpm run runfabric -- call-local -c examples/handler-scenarios/single-handler/runfabric.yml --serve --watch
curl -i http://127.0.0.1:8787/hello
```

## 2) Multiple handlers and triggers

Files:

- `multi-handler/runfabric.yml`
- `multi-handler/src/index.ts`
- `multi-handler/src/public.ts`
- `multi-handler/src/admin.ts`
- `multi-handler/src/webhook.ts`

`multi-handler/runfabric.yml` defines:

- root handler (`entry: src/index.ts`) with `/health`
- named function `public-api` with `GET /public`
- named function `admin-api` with `POST /admin`
- named function `webhook` with `POST /webhook`

Deploy only one function:

```bash
pnpm run runfabric -- deploy function public-api -c examples/handler-scenarios/multi-handler/runfabric.yml
```

Local-call one function entry:

```bash
pnpm run runfabric -- call-local -c examples/handler-scenarios/multi-handler/runfabric.yml --entry src/public.ts --path /public --method GET
pnpm run runfabric -- call-local -c examples/handler-scenarios/multi-handler/runfabric.yml --entry src/admin.ts --path /admin --method POST --body '{"role":"owner"}'
```

## 3) Framework wrappers (Express/Fastify/NestJS)

Use one wrapper API and pass your app instance:

```ts
import type { UniversalHandler } from "@runfabric/core";
import { createHandler } from "@runfabric/runtime-node";

export const handler: UniversalHandler = createHandler(appOrFastifyOrNestApp);
```

Express catch-all route pattern:

```ts
import express from "express";
import { createHandler } from "@runfabric/runtime-node";

const app = express();
app.use(express.json());
app.all("*", (req, res) => {
  res.status(200).json({
    route: req.originalUrl,
    method: req.method
  });
});

export const handler = createHandler(app);
```
