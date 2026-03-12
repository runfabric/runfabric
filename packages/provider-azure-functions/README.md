# @runfabric/provider-azure-functions

Azure Functions provider adapter for RunFabric.

## Install

```bash
npm install @runfabric/provider-azure-functions @runfabric/core
```

## Usage

```ts
import { createAzureFunctionsProvider } from "@runfabric/provider-azure-functions";

const provider = createAzureFunctionsProvider({ projectDir: process.cwd() });
```

In normal usage this package is loaded dynamically by `@runfabric/cli`.
