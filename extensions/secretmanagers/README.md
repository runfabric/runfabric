# Secret Manager Extensions

This directory contains external `kind=secret-manager` plugins that resolve secret manager references used in `runfabric.yml`.

Available plugins:

- `aws-secret-manager` (`aws-sm://...`)
- `gcp-secret-manager` (`gcp-sm://...`)
- `azure-key-vault-secret-manager` (`azure-kv://...`)
- `vault-secret-manager` (`vault://...`)

Each plugin exposes `ResolveSecret` and `GetSecret` over the RunFabric extension RPC protocol.

Example config:

```yaml
extensions:
  secretManagerPlugin: aws-secret-manager

secrets:
  db_password: aws-sm://prod/my-service/db-password?region=us-east-1
```
