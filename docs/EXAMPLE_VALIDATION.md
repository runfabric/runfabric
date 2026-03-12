# Example Validation Checklist

Use this checklist after scaffolding or editing example projects.

## 1. Name To Config Consistency

If directory name follows `runfabric-<provider>-<trigger>-state-<backend>`, verify:

- `<provider>` matches `providers[0]` in `runfabric.yml`.
- `<trigger>` matches `triggers[0].type` in `runfabric.yml`.
- `<backend>` matches `state.backend` in `runfabric.yml`.

## 2. Scaffold Template Limits

`init` scaffolds `api|worker|queue|cron` only.

For examples named `storage|eventbridge|pubsub`:

- scaffold from `worker`
- replace `triggers` in `runfabric.yml` with intended trigger type
- rerun `runfabric plan` to validate

## 3. Provider ID Consistency

Use exact provider IDs:

- `aws-lambda`
- `gcp-functions`
- `azure-functions`
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
  - `runfabric call-local -c runfabric.yml --serve --watch`
- Event-driven examples:
  - `runfabric call-local -c runfabric.yml --event ./event.json`
  - `runfabric dev -c runfabric.yml --preset queue --once`
  - `runfabric dev -c runfabric.yml --preset storage --once`

## 5. Framework Wrapper Consistency (Express/Fastify/Nest)

When example uses framework app wiring:

- `@runfabric/runtime-node` is installed
- framework dependencies are installed
- handler exports `createHandler(appOrFastifyOrNestApp)`
- Nest TS config enables `experimentalDecorators` and `emitDecoratorMetadata`

## 6. Documentation Sync

After changes:

- update example README command snippets
- ensure trigger examples in README match `runfabric.yml`
- keep `docs/EXAMPLES_MATRIX.md` aligned with planner capability matrix
- run `runfabric docs sync -c runfabric.yml` to refresh generated sections
