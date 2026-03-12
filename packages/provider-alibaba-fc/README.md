# @runfabric/provider-alibaba-fc

Alibaba Function Compute provider adapter for RunFabric.

## Install

```bash
npm install @runfabric/provider-alibaba-fc @runfabric/core
```

## Usage

```ts
import { createAlibabaFcProvider } from "@runfabric/provider-alibaba-fc";

const provider = createAlibabaFcProvider({ projectDir: process.cwd() });
```

In normal usage this package is loaded dynamically by `@runfabric/cli`.
