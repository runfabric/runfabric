# Router CI Template (`dev -> staging -> prod`)

This template is a production-minded baseline for routing automation:

- deploy per stage
- run drift check before apply
- enforce staged approvals (`dev -> staging -> prod`)
- require explicit risk approval for risky/high-volume DNS mutations

```yaml
name: Router Rollout

on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  deploy-dev:
    runs-on: ubuntu-latest
    environment: dev
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go install github.com/runfabric/runfabric/cmd/runfabric@latest
      - run: runfabric doctor --config runfabric.yml --stage dev
      - run: runfabric plan --config runfabric.yml --stage dev --json
      - run: runfabric router deploy --config runfabric.yml --stage dev --sync-dns
        env:
          RUNFABRIC_REAL_DEPLOY: "1"
          RUNFABRIC_ROUTER_API_TOKEN: ${{ secrets.RUNFABRIC_ROUTER_API_TOKEN_DEV }}
          RUNFABRIC_ROUTER_ZONE_ID: ${{ secrets.RUNFABRIC_ROUTER_ZONE_ID_DEV }}
          RUNFABRIC_ROUTER_ACCOUNT_ID: ${{ secrets.RUNFABRIC_ROUTER_ACCOUNT_ID_DEV }}
          RUNFABRIC_DNS_SYNC_REASON: "CI deploy to dev"
          RUNFABRIC_DNS_SYNC_RISK_APPROVED: "true"

  deploy-staging:
    needs: [deploy-dev]
    runs-on: ubuntu-latest
    environment: staging
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go install github.com/runfabric/runfabric/cmd/runfabric@latest
      - run: runfabric plan --config runfabric.yml --stage staging --json
      - run: runfabric router dns-reconcile --config runfabric.yml --stage staging --json
        env:
          RUNFABRIC_ROUTER_API_TOKEN: ${{ secrets.RUNFABRIC_ROUTER_API_TOKEN_STAGING }}
          RUNFABRIC_ROUTER_ZONE_ID: ${{ secrets.RUNFABRIC_ROUTER_ZONE_ID_STAGING }}
          RUNFABRIC_ROUTER_ACCOUNT_ID: ${{ secrets.RUNFABRIC_ROUTER_ACCOUNT_ID_STAGING }}
      - run: runfabric router deploy --config runfabric.yml --stage staging --sync-dns --enforce-dns-sync-stage-rollout
        env:
          RUNFABRIC_REAL_DEPLOY: "1"
          RUNFABRIC_ROUTER_API_TOKEN: ${{ secrets.RUNFABRIC_ROUTER_API_TOKEN_STAGING }}
          RUNFABRIC_ROUTER_ZONE_ID: ${{ secrets.RUNFABRIC_ROUTER_ZONE_ID_STAGING }}
          RUNFABRIC_ROUTER_ACCOUNT_ID: ${{ secrets.RUNFABRIC_ROUTER_ACCOUNT_ID_STAGING }}
          RUNFABRIC_DNS_SYNC_DEV_APPROVED: "true"
          RUNFABRIC_DNS_SYNC_REASON: "CI promote to staging"
          RUNFABRIC_DNS_SYNC_RISK_APPROVED: "true"

  deploy-prod:
    needs: [deploy-staging]
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go install github.com/runfabric/runfabric/cmd/runfabric@latest
      - run: runfabric plan --config runfabric.yml --stage prod --json
      - run: runfabric router dns-reconcile --config runfabric.yml --stage prod --json
        env:
          RUNFABRIC_ROUTER_API_TOKEN: ${{ secrets.RUNFABRIC_ROUTER_API_TOKEN_PROD }}
          RUNFABRIC_ROUTER_ZONE_ID: ${{ secrets.RUNFABRIC_ROUTER_ZONE_ID_PROD }}
          RUNFABRIC_ROUTER_ACCOUNT_ID: ${{ secrets.RUNFABRIC_ROUTER_ACCOUNT_ID_PROD }}
      - run: runfabric router deploy --config runfabric.yml --stage prod --sync-dns --allow-prod-dns-sync --enforce-dns-sync-stage-rollout
        env:
          RUNFABRIC_REAL_DEPLOY: "1"
          RUNFABRIC_ROUTER_API_TOKEN: ${{ secrets.RUNFABRIC_ROUTER_API_TOKEN_PROD }}
          RUNFABRIC_ROUTER_ZONE_ID: ${{ secrets.RUNFABRIC_ROUTER_ZONE_ID_PROD }}
          RUNFABRIC_ROUTER_ACCOUNT_ID: ${{ secrets.RUNFABRIC_ROUTER_ACCOUNT_ID_PROD }}
          RUNFABRIC_DNS_SYNC_STAGING_APPROVED: "true"
          RUNFABRIC_DNS_SYNC_REASON: "CI promote to production"
          RUNFABRIC_DNS_SYNC_RISK_APPROVED: "true"
```

Notes:

- Prefer short-lived tokens from your CI OIDC + secret manager flow.
- Use `extensions.router.credentialPolicy` and set attestation envs (for example `RUNFABRIC_ROUTER_TOKEN_ATTESTED`, `RUNFABRIC_ROUTER_TOKEN_ISSUED_AT`, `RUNFABRIC_ROUTER_TOKEN_EXPIRES_AT`) to enforce token freshness in CI.
- You can mount tokens as files and use `RUNFABRIC_ROUTER_API_TOKEN_FILE`.
- Or use first-class secret refs via `extensions.router.credentials.apiTokenSecretRef`.
- Use `extensions.router.mutationPolicy` in `runfabric.yml` to require `RUNFABRIC_DNS_SYNC_RISK_APPROVED=true` for risky changes.
- A runnable manual pipeline is available in `.github/workflows/router-lifecycle.yml`.
