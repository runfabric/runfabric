# SDK framework support

RunFabric SDKs let you run the same handler logic in your preferred framework (Node, Python, or Go) for local dev and deployment.

## Quick navigation

- **Node**: `@runfabric/sdk` handlers + adapters
- **Python**: framework mounts + raw ASGI/WSGI
- **Go/Java/.NET**: handler + HTTP wrappers
- **Hook parity**: lifecycle hooks across SDKs

## Node (`@runfabric/cli` + `@runfabric/sdk`)

| Support        | Description |
|----------------|-------------|
| **Single handler** | `createHandler(handler)` – one function that manages internally; use as HTTP (req, res) or call `.mountExpress()`, `.mountFastify()`, `.forNest()`. |
| **Express**    | Mount handlers as Express routes via `@runfabric/sdk/adapters/express`. |
| **Fastify**   | Mount handlers as Fastify routes via `@runfabric/sdk/adapters/fastify`. |
| **Nest**      | Use handlers in Nest.js via `@runfabric/sdk/adapters/nest`. |
| **Raw handler** | `(event, context) => response`; wrap with `createHttpHandler` from `@runfabric/sdk/adapters/raw`. |

**Example (single handler – recommended):**

One handler function, one entry point; use as raw HTTP or mount on any framework:

```js
const http = require("http");
const { createHandler } = require("@runfabric/sdk");

const myHandler = (event, context) => ({ message: "hello", stage: context.stage });
const h = createHandler(myHandler);

// Use as raw HTTP server
http.createServer(h).listen(3000);

// Or mount on Express (same h)
// const express = require("express");
// const app = express();
// h.mountExpress(app, "/api", "post");

// Or mount on Fastify (same h)
// h.mountFastify(fastify, { url: "/api", method: "POST" });

// Or use in Nest (same h)
// const nestHandler = h.forNest();
```

**Example (Express only):**

```js
const express = require("express");
const { mount } = require("@runfabric/sdk/adapters/express");

const app = express();
app.use(express.json());
const myHandler = (event, context) => ({ message: "hello", stage: context.stage });
mount(app, myHandler, "/api", "post");
app.listen(3000);
```

**Example (raw HTTP only):**

```js
const { createHttpHandler } = require("@runfabric/sdk/adapters/raw");
const handler = (event, context) => ({ ok: true });
const httpHandler = createHttpHandler(handler);
// use httpHandler(req, res) in any Node HTTP server
```

## Hook parity

- **Node SDK**: lifecycle hook contract is implemented (`beforeBuild`, `afterBuild`, `beforeDeploy`, `afterDeploy`) via `loadHookModules` and `runLifecycleHooks`.
- **Python / Go / Java / .NET SDKs**: handler and HTTP adapter parity is implemented; lifecycle hook execution is not part of SDK runtime execution yet.
- **Engine boundary**: Go engine treats Node hook modules as addon hooks and provider/runtime/simulator plugins as Go-side extension contracts.

---

## Python (`packages/python/runfabric`, `runfabric`)

| Support        | Description |
|----------------|-------------|
| **FastAPI**   | Mount handlers with `runfabric.adapters.fastapi_mount(app, handler, path=..., method=...)`. |
| **Flask**     | Mount with `runfabric.adapters.flask_mount(app, handler, path=..., methods=[...])`. |
| **Django**    | Use `runfabric.adapters.runfabric_view(handler)` in `urlpatterns`. |
| **Raw ASGI/WSGI** | `runfabric.adapters.create_asgi_handler(handler)` or `create_wsgi_handler(handler)`. |

**Example (FastAPI):**

```python
from fastapi import FastAPI
from runfabric.adapters import fastapi_mount

app = FastAPI()
def my_handler(event, context):
    return {"message": "hello", "stage": context.get("stage", "dev")}
fastapi_mount(app, my_handler, path="/api", method="post")
```

**Example (raw ASGI):**

```python
from runfabric.adapters import create_asgi_handler
handler = lambda event, context: {"ok": True}
asgi_app = create_asgi_handler(handler)
# use asgi_app in uvicorn or any ASGI server
```

---

## Go (`packages/go/sdk`)

| Support        | Description |
|----------------|-------------|
| **Handler**   | `Handler func(ctx, event, runCtx) (map[string]any, error)`; use `handler.Func(fn)` for simple `(event, context) -> response`. |
| **HTTP**      | `handler.HTTPHandler(h)` implements `http.Handler` for use with `net/http` or any HTTP framework. |

**Example:**

```go
import "github.com/runfabric/runfabric/sdk/go/handler"

h := handler.Func(func(event map[string]any, runCtx *handler.Context) map[string]any {
    return map[string]any{"message": "hello", "stage": runCtx.Stage}
})
http.ListenAndServe(":3000", handler.HTTPHandler(h))
```

**Build and test:** `cd packages/go/sdk && go test ./...`

---

## Java (`packages/java/sdk`, `io.runfabric:runfabric-sdk`)

| Support        | Description |
|----------------|-------------|
| **Handler**   | `Handler` interface: `Map<String, Object> handle(Map<String, Object> event, HandlerContext context)`. |
| **HTTP**      | `HttpHandler` wraps a handler for request/response streams (e.g. servlet or programmatic HTTP). |

**Example:**

```java
import dev.runfabric.Handler;
import dev.runfabric.HandlerContext;

Handler h = (event, context) -> Map.of("message", "hello", "stage", context.getStage());
```

**Build and test:** `cd packages/java/sdk && mvn clean test`

---

## .NET (`packages/dotnet/sdk`, RunFabric.Sdk)

| Support        | Description |
|----------------|-------------|
| **Handler**   | `Handler` delegate: `(event, context) -> IReadOnlyDictionary<string, object?>`. |
| **HTTP**      | `HttpHandler.InvokeAsync(requestStream, responseStream, handler, ...)` for stream-based HTTP. |

**Example:**

```csharp
using RunFabric.Sdk;

Handler h = (event, context) => new Dictionary<string, object?>
{
    ["message"] = "hello",
    ["stage"] = context.Stage
};
```

**Build and test:** `cd packages/dotnet/sdk && dotnet test`
