# @runfabric/provider-vercel

Vercel provider adapter for RunFabric.

## Install

```bash
npm install @runfabric/provider-vercel @runfabric/core
```

## Usage

```ts
import { createVercelProvider } from "@runfabric/provider-vercel";

const provider = createVercelProvider({ projectDir: process.cwd() });
```

In normal usage this package is loaded dynamically by `@runfabric/cli`.
