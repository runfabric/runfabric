# Deploy by Provider (API only — no CLI)

`runfabric deploy` performs **real** deploys using each provider’s **REST API or SDK only**. No wrangler, vercel, fly, gcloud, kubectl, or other provider CLIs are required.

| Provider | How deploy runs | Required env / setup |
|----------|-----------------|----------------------|
| **aws** / **aws-lambda** | AWS SDK (API Gateway, Lambda, triggers). | AWS credentials; no extra CLI. |
| **cloudflare-workers** | Cloudflare REST API: upload worker script. | `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_API_TOKEN`. Build worker (e.g. `worker.js` or `dist/worker.js`) first. |
| **vercel** | Vercel REST API: create deployment with file payload. | `VERCEL_TOKEN`; optional `VERCEL_TEAM_ID`. |
| **netlify** | Netlify REST API: create site (if needed), deploy zip. | `NETLIFY_AUTH_TOKEN`; optional `NETLIFY_SITE_ID` (created if missing). |
| **fly-machines** | Fly Machines REST API: create app. | `FLY_API_TOKEN`; optional `FLY_ORG_ID` (default `personal`). |
| **gcp-functions** | Google Cloud Functions REST API (v2). | `GCP_ACCESS_TOKEN`, `GCP_PROJECT`, `GCP_SOURCE_BUCKET`, `GCP_SOURCE_OBJECT` (upload app to GCS first). |
| **azure-functions** | Azure Management REST API: resource group + function app. | `AZURE_ACCESS_TOKEN`, `AZURE_SUBSCRIPTION_ID`; optional `AZURE_RESOURCE_GROUP`. |
| **kubernetes** | Kubernetes API (client-go): namespace, Deployment, Service. | Kubeconfig (`KUBECONFIG` or `~/.kube/config`); in-cluster config when inside a cluster. |
| **digitalocean-functions** | DigitalOcean App Platform REST API: **functions** (HTTP) + **jobs** with `kind: SCHEDULED` (cron). | `DIGITALOCEAN_ACCESS_TOKEN`, `DO_APP_REPO` (e.g. `owner/repo`); optional `DO_REGION` (default `ams`). |
| **alibaba-fc** | Alibaba FC OpenAPI (signed): CreateService, CreateFunction (zip), CreateTrigger (http/cron/queue/storage per capability matrix). | `ALIBABA_ACCESS_KEY_ID`, `ALIBABA_ACCESS_KEY_SECRET`, `ALIBABA_FC_ACCOUNT_ID`; optional `ALIBABA_FC_REGION` (default `cn-hangzhou`). |
| **ibm-openwhisk** | OpenWhisk REST API: create/update actions. | `IBM_OPENWHISK_AUTH` (e.g. `user:password` or API key); optional `IBM_OPENWHISK_API_HOST`, `IBM_OPENWHISK_NAMESPACE`. |

## Triggers

- **HTTP**: Supported by all providers via the deploy above (worker/app/function URL).
- **Cron / queue / storage / eventbridge / pubsub**: Configured in provider-native config. Use the same `runfabric.yml` events; deploy creates the app; add trigger config in the generated or existing provider config for full trigger support.

## DigitalOcean: function + cron

For `digitalocean-functions`, deploy creates one **function** (HTTP) and one **job** per cron event. Example `runfabric.yml`:

```yaml
service: my-api
provider:
  name: digitalocean-functions
  runtime: nodejs
functions:
  api:
    handler: index.handler
    events:
      - http: { path: /, method: get }
      - cron: "0 * * * *"   # hourly
```

Set `DIGITALOCEAN_ACCESS_TOKEN` and `DO_APP_REPO` (e.g. `owner/repo`). The cron job runs `node index.js` on the schedule; ensure your app handles the cron invocation (e.g. via env or a dedicated entry point).

## Prerequisites

1. **Provider** in `runfabric.yml`: `provider.name` set to one of the names above.
2. **Env vars** for that provider (see table); no CLI tools required.
3. **Project layout** suitable for the provider (e.g. Node app for Vercel/Netlify, worker entry for Cloudflare).

After deploy, a receipt is written to `.runfabric/<stage>.json` with outputs (e.g. URL).

## Engine and CLI support

The code engine supports all lifecycle commands via the CLI:

| Command | Behavior |
|---------|----------|
| **deploy** | AWS: control plane + SDK. Other providers: `internal/deploy/api.Run` (REST/SDK), no provider CLI. |
| **remove** | AWS: control plane + provider Remove. Other providers: `internal/deploy/api.Remove` (delete via provider API, then clear receipt). |
| **doctor** | Registry provider (stub or AWS). |
| **plan** | Registry provider; trigger validation against capability matrix. |
| **invoke** | AWS: registry. Other providers: `internal/deploy/api.Invoke` (HTTP POST to deployed URL, or OpenWhisk API for IBM). |
| **logs** | AWS: registry. Other providers: `internal/deploy/api.Logs` (Fly/Vercel/Netlify/DO/Cloudflare/IBM logs API or console link). |
| **list** | From config + receipt (no provider call). |
| **traces** / **metrics** | Receipt + message; use provider console for full data. |
| **call-local** / **dev** / **test** / **debug** | Local only; no provider. |
| **init** / **build** / **package** / **migrate** / **state** | Config and tooling; no deploy/remove. |
