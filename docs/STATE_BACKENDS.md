# State Backends

This document defines how runfabric state storage works, how to wire backend credentials, and minimum access requirements. Aligned with [upstream STATE_BACKENDS](https://github.com/runfabric/runfabric/blob/main/docs/STATE_BACKENDS.md). In this repo the Go engine supports `backend` (legacy) and `state` (reference format in runfabric.yml); config is normalized so both map to the same backends.

Quick credentials matrix (providers + state backends): [CREDENTIALS.md](CREDENTIALS.md).

## Supported Backends

- `local` (default)
- `postgres` — receipts stored in Postgres; set `backend.postgresConnectionStringEnv` (env var name for DSN) and optional `backend.postgresTable` (default `runfabric_receipts`).
- `sqlite` — receipts in a SQLite file; set `backend.sqlitePath` (default `.runfabric/state.db`; resolved relative to project root).
- `dynamodb` — receipts in DynamoDB; set `backend.receiptTable` or `backend.lockTable`, and use provider region for AWS.
- `s3`
- `gcs`
- `azblob`
- `aws-remote` — S3 for receipts + DynamoDB for locks (existing).

`runfabric init` prompts for state backend selection and defaults to `local`.

**Implemented (1.5 / 1.6):** Deploy state (receipts) can use **Postgres**, **SQLite**, or **DynamoDB** in addition to local and S3. Set `backend.kind` to `postgres`, `sqlite`, or `dynamodb` and configure the connection (see below). Receipts are stored and fetched via the same backend; dashboard, metrics, traces, and list use it.

## Schema

```yaml
state:
  backend: local | postgres | s3 | gcs | azblob
  keyPrefix: runfabric/state
  lock:
    enabled: true
    timeoutSeconds: 30
    heartbeatSeconds: 10
    staleAfterSeconds: 60
  local:
    dir: ./.runfabric/state
  postgres:
    connectionStringEnv: RUNFABRIC_STATE_POSTGRES_URL
    schema: public
    table: runfabric_state
  s3:
    bucket: my-state-bucket
    region: us-east-1
    keyPrefix: runfabric/state
    useLockfile: true
  gcs:
    bucket: my-state-bucket
    prefix: runfabric/state
  azblob:
    container: runfabric-state
    prefix: runfabric/state
```

You can use dynamic env bindings in these values:

- `${env:VAR_NAME}`
- `${env:VAR_NAME,default-value}`

## Credential Wiring

### local

- No cloud credential required.

### postgres (receipts)

- Set connection string env named by `backend.postgresConnectionStringEnv` or `state.postgres.connectionStringEnv` (default `RUNFABRIC_STATE_POSTGRES_URL`).
- Table name: `backend.postgresTable` (default `runfabric_receipts`). Table is created automatically with columns `workspace_id`, `stage`, `data` (JSONB), `updated_at`.
- Example: `export RUNFABRIC_STATE_POSTGRES_URL="postgres://user:pass@host:5432/dbname?sslmode=require"`

### sqlite (receipts)

- Set `backend.sqlitePath` (default `.runfabric/state.db`); path is relative to project root. Table `runfabric_receipts` is created automatically.

### dynamodb (receipts)

- Set `backend.receiptTable` (or `backend.lockTable`) and ensure provider has `region`. Table must have partition key `pk` (String) and sort key `sk` (String). Items: `pk` = workspace ID (root path), `sk` = `STAGE#<stage>`, `data` = receipt JSON string, `updatedAt` = timestamp.

### s3

- Use AWS credential chain (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, optional `AWS_SESSION_TOKEN`) and region.
- Example:

```bash
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_REGION="us-east-1"
```

### gcs

- Use service account credentials (`GOOGLE_APPLICATION_CREDENTIALS`) or workload identity.

### azblob

- Use one of:
  - `AZURE_STORAGE_CONNECTION_STRING`
  - `AZURE_STORAGE_ACCOUNT` + `AZURE_STORAGE_KEY`
  - managed identity/service principal envs.

## Minimum Permissions

### postgres

- Table DDL once (or pre-provision table): `CREATE TABLE`, `CREATE INDEX` (if bootstrap enabled).
- Runtime operations: `SELECT`, `INSERT`, `UPDATE`, `DELETE`.

### s3

- Bucket/object operations for state key prefix:
  - `s3:GetObject`
  - `s3:PutObject`
  - `s3:DeleteObject`
  - `s3:ListBucket` (scoped to prefix)

### gcs

- Bucket/object operations for state prefix:
  - `storage.objects.get`
  - `storage.objects.create`
  - `storage.objects.delete`
  - `storage.objects.list`

### azblob

- Container/blob operations for state prefix:
  - read/write/delete/list blob permissions
  - lease/lock equivalent permissions if using lock objects

## Security Controls

### Encryption Expectations

- At rest:
  - `local`: rely on host disk encryption policy.
  - `postgres`: enable database encryption-at-rest on managed service or volume encryption.
  - `s3`: enable SSE-S3 or SSE-KMS on bucket/prefix.
  - `gcs`: default Google-managed encryption or CMEK.
  - `azblob`: storage encryption enabled (Microsoft-managed or CMK).
- In transit:
  - require TLS for DB/object-store transport (`https`, `sslmode=require`, private endpoints where possible).

### Secret Redaction

- runfabric redacts credential-like keys before writing `details` to state.
- Any field name matching sensitive patterns (`secret`, `token`, `password`, `credential`, `apiKey`, etc.) is persisted as `[REDACTED]`.

## Operational Commands

```bash
runfabric state list -c runfabric.yml --json
runfabric state pull -c runfabric.yml --provider aws-lambda --json
runfabric state backup -c runfabric.yml --out ./.runfabric/backup/state.json --json
runfabric state restore -c runfabric.yml --file ./.runfabric/backup/state.json --json
runfabric state reconcile -c runfabric.yml --json
runfabric state force-unlock -c runfabric.yml --service my-svc --stage dev --provider aws-lambda --json
runfabric state migrate -c runfabric.yml --from local --to postgres --json
```

## Notes On Current Runtime Behavior

- `local` uses `.runfabric/state/<service>/<stage>/<provider>.state.json`.
- `postgres` uses a real table backend (`state.postgres.schema`.`state.postgres.table`) keyed by `<keyPrefix>/<service>/<stage>/<provider>.state.json`.
- `s3`, `gcs`, and `azblob` use real object storage backends keyed by `<prefix>/<service>/<stage>/<provider>.state.json`.
- Locking is token-based with timeout, stale-lock recovery, and heartbeat renewal.

## Recovering from partial deploys and journal conflicts

**Runbook-style steps** when state or locking gets out of sync:

1. **Partial deploy (deploy started but failed mid-way)**  
   - Run `runfabric plan` to see current vs desired state.  
   - Re-run `runfabric deploy`; the engine is designed to converge. If the provider left resources behind, remove them manually in the cloud console or run `runfabric remove` and then deploy again.  
   - For AWS with journaling: if a deploy wrote journal entries but never completed, consider `runfabric recover --dry-run` then `runfabric recover` to reconcile or roll back.

2. **Journal or lock conflict (e.g. "stale lock", "version conflict")**  
   - Ensure only one process is deploying to the same stage/provider at a time.  
   - If a previous process crashed while holding a lock: use `runfabric state force-unlock` with the same `--service`, `--stage`, and `--provider` to clear the lock, then retry deploy.  
   - For DynamoDB/S3 backends, check that the receipt table and lock table (if used) are writable and that no other tool is writing to the same keys.

3. **State migrate (moving from local to postgres/s3/etc.)**  
   - Run `runfabric state backup` with the current backend to export state.  
   - Configure the new backend in `runfabric.yml`.  
   - Run `runfabric state restore` from the backup file.  
   - Run `runfabric state reconcile` to align with the provider if needed.  
   - See `runfabric state migrate` for a single-command migration path when supported.

4. **Reconcile (state and cloud out of sync)**  
   - `runfabric state reconcile` compares local state with the provider and can report drift. Use it after manual changes in the cloud or after restoring from backup.

## Test Coverage Notes

- Remote backend integration tests are opt-in and gated by `RUNFABRIC_TEST_REMOTE_STATE=1`.
- Default test runs always cover local backend behavior; remote tests are skipped unless explicitly enabled with valid credentials.
