# CI Templates

This document ships ready-to-copy CI workflow templates for preview and promotion pipelines.

Templates:

- `.github/workflows/templates/preview.yml`
- `.github/workflows/templates/promotion.yml`

These are templates only. Copy or adapt them into active workflow files under `.github/workflows/*.yml`.

## 1) Preview Deployments Per PR

Template: `.github/workflows/templates/preview.yml`

Purpose:

- run checks on pull requests
- deploy a preview environment (example: AWS Lambda, stage `dev`)
- collect traces and metrics for the preview deploy

Recommended pre-deploy checks include:

- `pnpm run check:syntax`
- `pnpm run check:capabilities`
- `pnpm run check:compatibility`

Key runfabric commands:

- `runfabric deploy -c runfabric.yml --stage dev --json`
- `runfabric traces --provider aws-lambda --json`
- `runfabric metrics --provider aws-lambda --json`

## 2) Promotion Flow (`dev -> staging -> prod`)

Template: `.github/workflows/templates/promotion.yml`

Purpose:

- manually promote artifacts/config through environments
- run validation before each promotion
- target either `staging` or `prod` via `workflow_dispatch` input

Recommended model:

1. PR preview deploys to `dev`.
2. promotion workflow deploys to `staging`.
3. production promotion is explicit and auditable.

## 3) Provider Credential Wiring Per Environment

Example names (AWS):

- Preview (`dev`):
  - `AWS_ACCESS_KEY_ID_PREVIEW`
  - `AWS_SECRET_ACCESS_KEY_PREVIEW`
  - `RUNFABRIC_AWS_LAMBDA_ROLE_ARN_PREVIEW`
  - optional: `RUNFABRIC_AWS_DEPLOY_CMD_PREVIEW`
  - `AWS_REGION_PREVIEW` (as variable)
- Staging:
  - `AWS_ACCESS_KEY_ID_STAGING`
  - `AWS_SECRET_ACCESS_KEY_STAGING`
  - `RUNFABRIC_AWS_LAMBDA_ROLE_ARN_STAGING`
  - optional: `RUNFABRIC_AWS_DEPLOY_CMD_STAGING`
  - `AWS_REGION_STAGING` (as variable)
- Production:
  - `AWS_ACCESS_KEY_ID_PROD`
  - `AWS_SECRET_ACCESS_KEY_PROD`
  - `RUNFABRIC_AWS_LAMBDA_ROLE_ARN_PROD`
  - optional: `RUNFABRIC_AWS_DEPLOY_CMD_PROD`
  - `AWS_REGION_PROD` (as variable)

Guidance:

- keep stage credentials isolated per GitHub environment
- protect production with required reviewers
- avoid sharing deploy command secrets across stages

## 4) Correlation and Observability in CI

After deploy, collect traces/metrics to correlate deploy and invoke activity:

```bash
pnpm run runfabric -- traces --provider aws-lambda --json
pnpm run runfabric -- metrics --provider aws-lambda --json
```

The trace/metric outputs are derived from local provider receipts and event logs written under `.runfabric/deploy/<provider>/`.

## 5) Security Scanning (Snyk)

Active workflow: `.github/workflows/snyk.yml`

Required secret:

- `SNYK_TOKEN`

Checks run:

- `pnpm run security:snyk:test` on PRs and pushes
- `pnpm run security:snyk:monitor` on `main` pushes (and scheduled runs)

Local run:

```bash
export SNYK_TOKEN="your-token"
pnpm run security:snyk:test
```
