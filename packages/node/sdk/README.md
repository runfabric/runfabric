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

require("http").createServer(h).listen(3000); // raw HTTP
h.mountExpress(app, "/api", "post"); // Express
h.mountFastify(fastify, { url: "/api" }); // Fastify
const nestHandler = h.forNest(); // Nest
```

## App as handler (Express / Fastify / Nest)

**universalHandler** (used by `createHandler(app)`) binds an existing Express, Fastify, or Nest app to the Lambda/RunFabric invocation model. Pass your framework app to get a single handler for `runfabric.yml`:

```js
const { createHandler } = require("@runfabric/sdk");
const app = require("express")(); // or Fastify(), or Nest app
app.use(require("express").json());
app.post("/", (req, res) => res.json({ ok: true }));

const handler = createHandler(app); // (event, context) => response for RunFabric
module.exports = { handler };
```

TypeScript:

```ts
import type { UniversalHandler } from "@runfabric/sdk";
import { createHandler } from "@runfabric/sdk";
import app from "./app"; // Express, Fastify, or Nest app

export const handler: UniversalHandler = createHandler(app);
```

## Lifecycle hooks

**Types:** `import type { LifecycleHook, BuildHookContext, DeployHookContext } from "@runfabric/sdk"` to type your hook module.

**Execution:** The SDK provides `loadHookModules(hookPaths, cwd)` and `runLifecycleHooks(hookModules, phase, context)` so the CLI (or any runner) can load and run hooks:

```js
const { loadHookModules, runLifecycleHooks } = require("@runfabric/sdk");

const hooks = await loadHookModules(["./hooks.mjs"], process.cwd());
await runLifecycleHooks(hooks, "beforeBuild", { cwd: process.cwd(), config });
// ... build ...
await runLifecycleHooks(hooks, "afterBuild", { cwd: process.cwd(), config });
```

See [apps/registry/docs/PLUGINS.md](../../../apps/registry/docs/PLUGINS.md).

## Exports

| Import                            | Use                                                                                                                                                            |
| --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `@runfabric/sdk`                  | `createHandler`, `createHttpHandler`, `loadHandler`, `loadHookModules`, `runLifecycleHooks`, `PHASES`, `raw`, `express`, `fastify`, `nest`, `universalHandler` |
| `@runfabric/sdk/adapters`         | Same as above                                                                                                                                                  |
| `@runfabric/sdk/adapters/express` | `mount(app, handler, path, method)`                                                                                                                            |
| `@runfabric/sdk/adapters/fastify` | `register(fastify, handler, options)`                                                                                                                          |
| `@runfabric/sdk/adapters/nest`    | `nestHandler(handler)`                                                                                                                                         |
| `@runfabric/sdk/adapters/raw`     | `createHttpHandler(handler)`, `loadHandler(modulePath)`                                                                                                        |

See examples in the `examples/` directory for usage patterns.
