# Migration

Guidance for migrating to RunFabric from other setups.

## Quick navigation

- **Serverless Framework → RunFabric**: From Serverless Framework

## From Serverless Framework

RunFabric does not ship a top-level `runfabric migrate` command. Convert configs manually using this checklist:

1. Create a new `runfabric.yml` using `runfabric init --template http --provider <provider-id>`.
2. Copy service-level metadata (`service`, env, params, secrets) from your old config.
3. Map provider settings into:
   - `provider.name`, `provider.runtime`, `provider.region`
   - `backend` (and optional state backend detail blocks when applicable)
4. Convert each Serverless function into a `functions` array entry:
   - `name`
   - `entry`
   - `runtime` (optional per-function override)
   - `triggers` (`http|queue|cron|storage|eventbridge|pubsub`)
5. Recreate resources/addons/extensions using RunFabric-native keys.
6. Validate with:
   - `runfabric doctor -c runfabric.yml`
   - `runfabric plan -c runfabric.yml`

After migration, run a non-production deploy to verify runtime behavior before promoting.

**See also:** [RUNFABRIC_YML_REFERENCE.md](RUNFABRIC_YML_REFERENCE.md), [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md).
