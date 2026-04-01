# azure-key-vault-secret-manager extension

External `kind=secret-manager` plugin that resolves `azure-kv://...` references via Azure Key Vault.

This plugin shells out to `az`, so the environment must have authenticated Azure CLI access.

## Build

```bash
go build -o bin/azure-key-vault-secret-manager ./extensions/secretmanagers/azure
```

## Install (local)

Copy this folder to:

```text
$RUNFABRIC_HOME/plugins/secret-managers/azure-key-vault-secret-manager/0.1.0/
```

## Configure

```yaml
extensions:
  secretManagerPlugin: azure-key-vault-secret-manager

secrets:
  db_password: azure-kv://my-vault/prod-db-password
```

## Reference format

- Scheme: `azure-kv://`
- Supported forms:
  - `azure-kv://<vault>/<secret>`
  - `azure-kv://<vault>/<secret>/<version>`
  - `azure-kv://<secret>?vault=<vault>&version=<version>`
- Query params (optional):
  - `vault` (or `vaultName`)
  - `secret`
  - `version`
  - `jsonKey` (extract from JSON secret payload)

If vault is omitted, the plugin falls back to `AZURE_KEY_VAULT_NAME`.
