# @runfabric/core

Core contracts, types, enums, provider interfaces, and state backend primitives for RunFabric.

## Install

```bash
npm install @runfabric/core
```

## What it provides

- `ProjectConfig`, trigger/resource/state schema types
- `ProviderAdapter` and capability/credential contracts
- Provider IDs, enums, and portability primitives
- State backend abstractions (`local`, `postgres`, `s3`, `gcs`, `azblob`)

## Usage

```ts
import type { UniversalHandler } from "@runfabric/core";

export const handler: UniversalHandler = async () => ({
  status: 200,
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ ok: true })
});
```

See repository docs for end-to-end usage with the CLI and providers.
