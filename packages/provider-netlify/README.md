# @runfabric/provider-netlify

Netlify provider adapter for RunFabric.

## Install

```bash
npm install @runfabric/provider-netlify @runfabric/core
```

## Usage

```ts
import { createNetlifyProvider } from "@runfabric/provider-netlify";

const provider = createNetlifyProvider({ projectDir: process.cwd() });
```

In normal usage this package is loaded dynamically by `@runfabric/cli`.
