# Router Operations Workflow

This is the standard code-only operator flow for global router rollout.

## 1) Bootstrap

1. Set router credentials (prefer short-lived secret-manager sourced values):
   - `RUNFABRIC_ROUTER_API_TOKEN` (or `extensions.router.credentials.apiTokenSecretRef`)
   - `RUNFABRIC_ROUTER_ZONE_ID`
   - `RUNFABRIC_ROUTER_ACCOUNT_ID` (optional for DNS-only mode)
2. Set change-reason and approvals when policy requires them:
   - `RUNFABRIC_DNS_SYNC_REASON`
   - stage approval envs (for rollout gates)

## 2) Deploy

```bash
runfabric router deploy -c <config> --stage <stage>
```

## 3) Dry-run Drift/Policy Check

```bash
runfabric router dns-reconcile -c <config> --stage <stage>
```

This reports create/update/no-op/delete-candidate actions and prints trend analytics from sync history.

## 4) Apply

```bash
runfabric router dns-reconcile -c <config> --stage <stage> --apply
```

For `prod`, include `--allow-prod-dns-sync` and rollout approval envs as required by policy.

## 5) Verify

```bash
runfabric router status -c <config> --stage <stage>
runfabric router dns-history -c <config> --stage <stage>
runfabric router simulate -c <config> --stage <stage> --requests 500
runfabric router chaos-verify -c <config> --stage <stage>
```

`dns-history` includes operation IDs, before/after summaries, and trend windows for audit/replay.

## 6) Progressive Traffic Shift (Canary)

```bash
runfabric router dns-shift -c <config> --stage <stage> --provider <endpoint-name> --percent 20 --dry-run
runfabric router dns-shift -c <config> --stage <stage> --provider <endpoint-name> --percent 20
```

Use repeated `dns-shift` calls (for example 10 -> 20 -> 50 -> 80) for controlled rollout.

## 7) Rollback / Restore

Restore previous applied snapshot:

```bash
runfabric router dns-restore -c <config> --stage <stage>
```

Restore a specific snapshot:

```bash
runfabric router dns-restore -c <config> --stage <stage> --snapshot-id <id>
```

Preview restore first:

```bash
runfabric router dns-restore -c <config> --stage <stage> --dry-run
```

## 8) Troubleshooting

- `preflight: hostname is empty` or endpoint errors: fix `fabric.routing` and endpoint outputs.
- Token/identity failures: validate `extensions.router.credentials.*` and `extensions.router.credentialPolicy`.
- Unexpected drift: inspect `runfabric router dns-history` and `.runfabric/router-sync-<stage>.json`.
