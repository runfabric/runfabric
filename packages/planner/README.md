# @runfabric/planner

Config parsing, validation, portability diagnostics, and provider capability planning for RunFabric.

## Install

```bash
npm install @runfabric/planner @runfabric/core
```

## What it provides

- `parseProjectConfig` for `runfabric.yml`
- Capability matrix and primitive compatibility reports
- `createPlan` planning/validation output for providers and triggers

## Usage

```ts
import { parseProjectConfig, createPlan } from "@runfabric/planner";

const project = parseProjectConfig(yamlText, { projectDir: process.cwd() });
const planning = createPlan(project);
```
