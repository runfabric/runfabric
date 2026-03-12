# @runfabric/builder

Artifact build pipeline for RunFabric provider deployment flows.

## Install

```bash
npm install @runfabric/builder @runfabric/planner @runfabric/core
```

## What it provides

- `buildProject` to create provider-targeted artifacts
- Build manifests and runtime wrapper generation
- Deterministic output under `.runfabric/build/...`

## Usage

```ts
import { buildProject } from "@runfabric/builder";

await buildProject({
  planning,
  project,
  projectDir: process.cwd()
});
```
