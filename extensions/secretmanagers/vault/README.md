# vault-secret-manager extension

External `kind=secret-manager` plugin that resolves `vault://...` references via the Vault HTTP API.

## Build

```bash
go build -o bin/vault-secret-manager ./extensions/secretmanagers/vault
```

## Install (local)

Copy this folder to:

```text
$RUNFABRIC_HOME/plugins/secret-managers/vault-secret-manager/0.1.0/
```

## Configure

```yaml
extensions:
  secretManagerPlugin: vault-secret-manager

secrets:
  db_password: vault://secret/data/myapp?field=password
```

## Reference format

- Scheme: `vault://`
- Base: `vault://<path>`
- Query params (optional):
  - `field` (or `key`) to select a field from Vault response data
  - `addr` to override `VAULT_ADDR`
  - `namespace` to override `VAULT_NAMESPACE`

Environment requirements:

- `VAULT_TOKEN` (required)
- `VAULT_ADDR` (required unless `?addr=` is provided)
- `VAULT_NAMESPACE` (optional)
