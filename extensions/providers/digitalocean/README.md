# DigitalOcean provider

Deployment to DigitalOcean App Platform is implemented in **this folder** (actual API calls):

- **`deploy.go`** – Builds the app spec (functions + optional cron jobs), POST to `https://api.digitalocean.com/v2/apps`
- **`remove.go`** – DELETE app by ID from receipt
- **`invoke.go`** – HTTP POST to deployed app URL
- **`logs.go`** – GET app logs API

**Required env:** `DIGITALOCEAN_ACCESS_TOKEN`, `DO_APP_REPO` (e.g. `owner/repo`). Optional: `DO_REGION` (default `ams`).

`internal/deploy/api` registers this provider’s `Runner`, `Remover`, `Invoker`, and `Logger`; it does not contain DigitalOcean-specific logic. Run `runfabric deploy` with `provider.name: digitalocean-functions` in `runfabric.yml` to deploy.
