# @runfabric/runtime-node

Node runtime adapters and framework wrappers for RunFabric handlers.

## Install

```bash
npm install @runfabric/runtime-node @runfabric/core
```

## What it provides

- Provider-shaped adapter helpers for local/provider event bridging
- `createHandler` wrapper for Express/Fastify/Nest apps

## Usage

```ts
import type { UniversalHandler } from "@runfabric/core";
import { createHandler } from "@runfabric/runtime-node";

export const handler: UniversalHandler = createHandler(appOrFastifyOrNestApp);
```
