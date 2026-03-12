# @runfabric/provider-fly-machines

Fly Machines provider adapter for RunFabric.

## Install

```bash
npm install @runfabric/provider-fly-machines @runfabric/core
```

## Usage

```ts
import { createFlyMachinesProvider } from "@runfabric/provider-fly-machines";

const provider = createFlyMachinesProvider({ projectDir: process.cwd() });
```

In normal usage this package is loaded dynamically by `@runfabric/cli`.
