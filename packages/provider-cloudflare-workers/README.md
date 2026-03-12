# @runfabric/provider-cloudflare-workers

Cloudflare Workers provider adapter for RunFabric.

## Install

```bash
npm install @runfabric/provider-cloudflare-workers @runfabric/core
```

## Usage

```ts
import { createCloudflareWorkersProvider } from "@runfabric/provider-cloudflare-workers";

const provider = createCloudflareWorkersProvider({ projectDir: process.cwd() });
```

In normal usage this package is loaded dynamically by `@runfabric/cli`.
