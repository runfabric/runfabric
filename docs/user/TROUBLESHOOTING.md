# Troubleshooting

Common errors and fixes per provider and workflow.

## Quick navigation

- **Credentials missing**: General → “missing or empty …”
- **Trigger not supported**: General → “provider X does not support trigger Y”
- **Provider-specific issues**: jump to provider section

## General

### "missing or empty: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION"

- **Cause:** Provider credentials are required for real deploy/invoke.
- **Fix:** Set env vars (or use a profile). See [CREDENTIALS.md](CREDENTIALS.md) and [PROVIDER_SETUP.md](PROVIDER_SETUP.md).

### "provider X does not support trigger Y"

- **Cause:** The chosen provider does not support the trigger in the config (e.g. cron on fly-machines).
- **Fix:** Change trigger in `runfabric.yml` or switch provider. See [EXAMPLES_MATRIX.md](EXAMPLES_MATRIX.md) for support matrix.

### "functions must be a map to patch"

- **Cause:** `runfabric generate function` only patches when `functions` is a map in `runfabric.yml`.
- **Fix:** Use a map-style `functions:` (e.g. from `runfabric init`) or add the function entry manually.

---

## AWS (aws-lambda)

- **Missing env:** Set `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`. Optional: `AWS_SESSION_TOKEN` for temporary creds.
- **IAM / access denied:** Ensure the role/user has `lambda:*`, `apigateway:*`, `sqs:*`, etc. as needed. See [PROVIDER_SETUP.md](PROVIDER_SETUP.md).
- **Region:** Use a valid region (e.g. `us-east-1`). Wrong region can cause "resource not found" or timeout.

---

## GCP (gcp-functions)

- **Missing env:** Set `GCP_PROJECT_ID` and ensure Application Default Credentials (`gcloud auth application-default login`) or a service account key.
- **Wrong project:** Confirm `GCP_PROJECT_ID` matches the project in the console. Check billing and APIs (Cloud Functions API) are enabled.

---

## Azure (azure-functions)

- **Missing env:** Set `AZURE_SUBSCRIPTION_ID`, `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET` (or use Azure CLI login).
- **Resource group / region:** Ensure the app/function app exists in the expected resource group and region.

---

## Kubernetes

- **kubeconfig:** Set `KUBECONFIG` or use default `~/.kube/config`. Ensure the context has namespace and RBAC for creating deployments/services.
- **Image pull:** If using a private registry, ensure imagePullSecrets or cluster config is set.

---

## Cloudflare (cloudflare-workers)

- **Missing env:** Set `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`.
- **Real deploy:** Set `RUNFABRIC_CLOUDFLARE_REAL_DEPLOY=1` for real API deploy. See [PROVIDER_SETUP.md](PROVIDER_SETUP.md).

---

## Vercel / Netlify

- **Tokens:** Set the provider-specific token (e.g. `VERCEL_TOKEN`, `NETLIFY_AUTH_TOKEN`). See [PROVIDER_SETUP.md](PROVIDER_SETUP.md).
- **Project / site:** Ensure the project or site ID is correct for the account.

---

## Compose

- **"unsupported runtime" or deploy fails:** Each service in `runfabric.compose.yml` must point to a valid `runfabric.yml` with a supported provider and runtime (e.g. `nodejs20.x`). Fix the service config or runtime in the referenced `runfabric.yml`.
- **Concurrency:** Compose deploys services in dependency order; multiple services to the same provider run sequentially unless you use a higher `--concurrency` (see [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md)).

---

## See also

- [CREDENTIALS.md](CREDENTIALS.md) — credential resolution and env vars
- [PROVIDER_SETUP.md](PROVIDER_SETUP.md) — per-provider setup and optional env
- [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md) — flags and commands
