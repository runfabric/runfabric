# @runfabric/provider-digitalocean-functions

DigitalOcean Functions provider adapter for RunFabric.

## Install

```bash
npm install @runfabric/provider-digitalocean-functions @runfabric/core
```

## Usage

```ts
import { createDigitalOceanFunctionsProvider } from "@runfabric/provider-digitalocean-functions";

const provider = createDigitalOceanFunctionsProvider({ projectDir: process.cwd() });
```

In normal usage this package is loaded dynamically by `@runfabric/cli`.
