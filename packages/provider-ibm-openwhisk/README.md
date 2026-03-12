# @runfabric/provider-ibm-openwhisk

IBM OpenWhisk provider adapter for RunFabric.

## Install

```bash
npm install @runfabric/provider-ibm-openwhisk @runfabric/core
```

## Usage

```ts
import { createIbmOpenWhiskProvider } from "@runfabric/provider-ibm-openwhisk";

const provider = createIbmOpenWhiskProvider({ projectDir: process.cwd() });
```

In normal usage this package is loaded dynamically by `@runfabric/cli`.
