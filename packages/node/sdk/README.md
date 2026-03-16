# @runfabric/sdk

RunFabric SDK (Node) — handler contract, HTTP adapter, and framework adapters (Express, Fastify, Nest). For the CLI (`runfabric deploy`, etc.), use **@runfabric/cli**.

## Install

```bash
npm install @runfabric/sdk
```

## Single handler (recommended)

One function, one entry point — use as HTTP server or mount on any framework:

```js
const { createHandler } = require("@runfabric/sdk");
const h = createHandler((event, context) => ({ ok: true }));

require("http").createServer(h).listen(3000);  // raw HTTP
h.mountExpress(app, "/api", "post");            // Express
h.mountFastify(fastify, { url: "/api" });       // Fastify
const nestHandler = h.forNest();               // Nest
```

## Exports

| Import | Use |
|--------|-----|
| `@runfabric/sdk` | `createHandler`, `createHttpHandler`, `loadHandler`, `raw`, `express`, `fastify`, `nest` |
| `@runfabric/sdk/adapters` | Same as above |
| `@runfabric/sdk/adapters/express` | `mount(app, handler, path, method)` |
| `@runfabric/sdk/adapters/fastify` | `register(fastify, handler, options)` |
| `@runfabric/sdk/adapters/nest` | `nestHandler(handler)` |
| `@runfabric/sdk/adapters/raw` | `createHttpHandler(handler)`, `loadHandler(modulePath)` |

See [docs/SDK_FRAMEWORKS.md](../../docs/SDK_FRAMEWORKS.md) for examples.
