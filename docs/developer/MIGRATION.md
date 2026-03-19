# Migration

Guidance for migrating to RunFabric from other setups.

## Quick navigation

- **Serverless Framework → RunFabric**: From Serverless Framework

## From Serverless Framework

Use **`runfabric migrate`** to convert a `serverless.yml` to `runfabric.yml`:

```bash
runfabric migrate --input serverless.yml [--output runfabric.yml] [--provider <id>] [--dry-run] [--force]
```

- **--input** — path to existing `serverless.yml`
- **--output** — path for generated `runfabric.yml` (default: overwrite or stdout)
- **--provider** — target provider ID (e.g. `aws-lambda`)
- **--dry-run** — show what would be generated without writing
- **--force** — overwrite existing output file

After migration, run **`runfabric doctor`** and **`runfabric plan`** to validate.

**See also:** [RUNFABRIC_YML_REFERENCE.md](../user/RUNFABRIC_YML_REFERENCE.md), [COMMAND_REFERENCE.md](../user/COMMAND_REFERENCE.md).
