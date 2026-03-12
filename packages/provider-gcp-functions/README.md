# @runfabric/provider-gcp-functions

Google Cloud Functions provider adapter for RunFabric.

## Install

```bash
npm install @runfabric/provider-gcp-functions @runfabric/core
```

## Usage

```ts
import { createGcpFunctionsProvider } from "@runfabric/provider-gcp-functions";

const provider = createGcpFunctionsProvider({ projectDir: process.cwd() });
```

In normal usage this package is loaded dynamically by `@runfabric/cli`.
