# State Backends

This document defines how runfabric state storage works, how to wire backend credentials, and minimum access requirements.

## Supported Backends

- `local` (default)
- `postgres`
- `s3`
- `gcs`
- `azblob`

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

## Credential Wiring

### local

- No cloud credential required.

### postgres

- Set connection string env named by `state.postgres.connectionStringEnv` (default `RUNFABRIC_STATE_POSTGRES_URL`).
- Example:

```bash
export RUNFABRIC_STATE_POSTGRES_URL="postgres://user:pass@host:5432/dbname?sslmode=require"
```

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

## Test Coverage Notes

- Remote backend integration tests are opt-in and gated by `RUNFABRIC_TEST_REMOTE_STATE=1`.
- Default test runs always cover local backend behavior; remote tests are skipped unless explicitly enabled with valid credentials.
