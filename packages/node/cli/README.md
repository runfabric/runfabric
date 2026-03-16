# @runfabric/cli

RunFabric CLI — wrapper on the engine binary. Use `runfabric doctor`, `runfabric deploy`, etc.

## Install

```bash
npm install @runfabric/cli
```

Ensure the engine binary is available (bundled in releases, or run `make build-platform` in the repo and set `bin/` or `~/.runfabric/bin/`).

## Programmatic API

```js
const runfabric = require("@runfabric/cli");

runfabric.run("plan", ["--config", "runfabric.yml", "--stage", "dev"]);
runfabric.deploy("dev", "runfabric.yml", { rollbackOnFailure: false });
runfabric.build("dev", "runfabric.yml", "dist");
runfabric.inspect("dev", "runfabric.yml");
```

For handler contract and framework adapters (Express, Fastify, etc.), use **@runfabric/sdk**.
