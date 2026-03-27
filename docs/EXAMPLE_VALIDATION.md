# Example Validation Checklist

Use this checklist after scaffolding or editing example projects.

## Quick navigation

- **Naming conventions**: Name To Config Consistency
- **Scaffolding coverage**: Scaffold Template Coverage
- **Provider IDs**: Provider ID Consistency
- **Commands to run**: Validation Commands

## 1. Name To Config Consistency

If directory name follows `runfabric-<provider>-<trigger>-state-<backend>`, verify:

- `<provider>` matches `providers[0]` in `runfabric.yml`.
- `<trigger>` matches `triggers[0].type` in `runfabric.yml`.
- `<backend>` matches `state.backend` in `runfabric.yml`.

## 2. Scaffold Template Coverage

`init` scaffolds provider-supported trigger families:

- `api`
- `worker`
- `queue`
- `cron`
- `storage`
- `eventbridge`
- `pubsub`

`kafka` and `rabbitmq` remain valid trigger schema types but are hidden from `init` until provider capability support exists.

## 3. Provider ID Consistency

Use exact provider IDs (see [upstream list](https://github.com/runfabric/runfabric/blob/main/docs/EXAMPLE_VALIDATION.md) and `schemas/runfabric.schema.json`):

- `aws-lambda` (this engine also accepts `aws` as alias)
- `gcp-functions`
- `azure-functions`
- `kubernetes`
- `cloudflare-workers`
- `vercel`
- `netlify`
- `alibaba-fc`
- `digitalocean-functions`
- `fly-machines`
- `ibm-openwhisk`

## 4. Validation Commands

Run from example project root:

```bash
runfabric doctor -c runfabric.yml
runfabric plan -c runfabric.yml
runfabric build -c runfabric.yml
runfabric docs check -c runfabric.yml
```

Local execution:

- HTTP examples:
  - JavaScript scaffold script: `runfabric invoke local -c runfabric.yml --serve --watch`
  - TypeScript scaffold script: initial `tsc` build, then `tsc --watch` in parallel with `runfabric invoke local -c runfabric.yml --serve --watch`
- Event-driven examples:
  - `runfabric invoke local -c runfabric.yml --event ./event.json`
  - `runfabric invoke dev -c runfabric.yml --preset queue --once`
  - `runfabric invoke dev -c runfabric.yml --preset storage --once`

## 5. Framework Wrapper Consistency (Express/Fastify/Nest)

When example uses framework app wiring:

- `@runfabric/sdk` is installed
- framework dependencies are installed
- handler exports `createHandler(appOrFastifyOrNestApp)`
- Nest TS config enables `experimentalDecorators` and `emitDecoratorMetadata`

## 6. Documentation Sync

After changes:

- update example README command snippets
- ensure trigger examples in README match `runfabric.yml`
- keep `docs/EXAMPLES_MATRIX.md` aligned with planner capability matrix
- run `runfabric docs sync -c runfabric.yml` to refresh generated sections
