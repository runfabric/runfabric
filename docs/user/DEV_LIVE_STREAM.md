# Dev mode: live stream of remote invocations to local

**Status:** Implemented. Local server + instructions; **auto-wire lifecycle hook** runs for all built-in API providers when both `--stream-from` and `--tunnel-url` are set. AWS and Cloudflare include full provider route rewrite with restore on exit (Ctrl+C). GCP includes conditional mutation plus optional gateway-owned route rewrite hooks. Azure, DigitalOcean, Fly, Kubernetes, Netlify, Vercel, Alibaba FC, and IBM OpenWhisk support reversible route rewrite through provider-specific gateway hooks. `--doctor-first` reports live-stream capability mode, tunnel validation, and whether provider-side mutation prerequisites are present. Complements existing `runfabric invoke dev` presets (http, queue, storage, cron, etc.).

## Quick navigation

- **Just run local server**: `runfabric invoke dev --stream-from <stage>`
- **Use tunnel with any provider**: add `--tunnel-url <https://...>`
- **Provider capabilities**: see the provider-specific table below

## Goal

Route invocations that would go to the **deployed** function (e.g. on AWS Lambda) to a **local** process instead (e.g. `runfabric invoke local --serve`), so you can debug with real events without deploying every change.

## Proposed UX

```bash
# Terminal 1: start local receiver (serves HTTP and optionally registers with provider)
runfabric invoke dev --stream-from dev

# Optional: use a tunnel URL (e.g. ngrok, cloudflared) so the provider can reach your machine
runfabric invoke dev --stream-from dev --tunnel-url https://abc.ngrok.io
```

- `--stream-from <stage>`: Stage to “stream” from; invocations targeting that stage are forwarded to the local process.
- When supported by the provider, the CLI may configure the provider to send events to the tunnel URL (e.g. Lambda function URL → tunnel, or EventBridge rule → webhook).

## Provider-specific behavior

| Provider               | Approach                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| ---------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| AWS Lambda             | **Implemented full rewrite:** when `--stream-from` and `--tunnel-url` are set, the CLI finds the HTTP API for the stage, creates an HTTP proxy integration to the tunnel URL, and points all routes at it; on exit (SIGINT), routes are restored and the temporary integration is deleted. Function-URL-only deployments (no HTTP API) are not auto-wired; deploy with HTTP events first.                                                                                                                                                                                                                                                                                                       |
| GCP Cloud Functions    | **Auto-wire lifecycle hook:** prepare/restore hooks always run. By default, provider-side mutation patches function environment variables (conditional mutation), not full route rewrite, and depends on token, project, region, and function access. Optional gateway-owned route rewrite is supported when both `GCP_DEV_STREAM_GATEWAY_SET_URL` and `GCP_DEV_STREAM_GATEWAY_RESTORE_URL` are configured; the CLI calls those hooks to apply and restore reversible gateway routing. Without prerequisites or gateway hooks, behavior remains lifecycle-only and you should use [Firebase Local Emulator](https://firebase.google.com/docs/functions/local-emulator) or manual tunnel wiring. |
| Cloudflare Workers     | **Implemented full rewrite (route-based):** when `--stream-from` and `--tunnel-url` are set and `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`, and `CLOUDFLARE_ZONE_ID` are available, the CLI creates a temporary proxy worker, repoints matching zone routes to it, then restores original routes and deletes the proxy worker on exit. If no matching route exists, the CLI can create a temporary route when `CLOUDFLARE_DEV_ROUTE_PATTERN` (or `stages.<stage>.http.domain.name`) is provided. Without prerequisites or a resolvable route pattern, behavior falls back to lifecycle-only with explicit diagnostics.                                                                     |
| Azure Functions        | **Conditional route rewrite:** lifecycle hooks always run. Reversible route rewrite is applied when `RUNFABRIC_DEV_STREAM_AZURE_FUNCTIONS_SET_URL` and `RUNFABRIC_DEV_STREAM_AZURE_FUNCTIONS_RESTORE_URL` are configured; otherwise behavior falls back to lifecycle-only mode with explicit diagnostics.                                                                                                                                                                                                                                                                                                                                                                                       |
| DigitalOcean Functions | **Conditional route rewrite:** lifecycle hooks always run. Reversible route rewrite is applied when `RUNFABRIC_DEV_STREAM_DIGITALOCEAN_FUNCTIONS_SET_URL` and `RUNFABRIC_DEV_STREAM_DIGITALOCEAN_FUNCTIONS_RESTORE_URL` are configured; otherwise behavior falls back to lifecycle-only mode with explicit diagnostics.                                                                                                                                                                                                                                                                                                                                                                         |
| Fly Machines           | **Conditional route rewrite:** lifecycle hooks always run. Reversible route rewrite is applied when `RUNFABRIC_DEV_STREAM_FLY_MACHINES_SET_URL` and `RUNFABRIC_DEV_STREAM_FLY_MACHINES_RESTORE_URL` are configured; otherwise behavior falls back to lifecycle-only mode with explicit diagnostics.                                                                                                                                                                                                                                                                                                                                                                                             |
| Kubernetes             | **Conditional route rewrite:** lifecycle hooks always run. Reversible route rewrite is applied when `RUNFABRIC_DEV_STREAM_KUBERNETES_SET_URL` and `RUNFABRIC_DEV_STREAM_KUBERNETES_RESTORE_URL` are configured; otherwise behavior falls back to lifecycle-only mode with explicit diagnostics.                                                                                                                                                                                                                                                                                                                                                                                                 |
| Netlify                | **Conditional route rewrite:** lifecycle hooks always run. Reversible route rewrite is applied when `RUNFABRIC_DEV_STREAM_NETLIFY_SET_URL` and `RUNFABRIC_DEV_STREAM_NETLIFY_RESTORE_URL` are configured; otherwise behavior falls back to lifecycle-only mode with explicit diagnostics.                                                                                                                                                                                                                                                                                                                                                                                                       |
| Vercel                 | **Conditional route rewrite:** lifecycle hooks always run. Reversible route rewrite is applied when `RUNFABRIC_DEV_STREAM_VERCEL_SET_URL` and `RUNFABRIC_DEV_STREAM_VERCEL_RESTORE_URL` are configured; otherwise behavior falls back to lifecycle-only mode with explicit diagnostics.                                                                                                                                                                                                                                                                                                                                                                                                         |
| Alibaba FC             | **Conditional route rewrite:** lifecycle hooks always run. Reversible route rewrite is applied when `RUNFABRIC_DEV_STREAM_ALIBABA_FC_SET_URL` and `RUNFABRIC_DEV_STREAM_ALIBABA_FC_RESTORE_URL` are configured; otherwise behavior falls back to lifecycle-only mode with explicit diagnostics.                                                                                                                                                                                                                                                                                                                                                                                                 |
| IBM OpenWhisk          | **Conditional route rewrite:** lifecycle hooks always run. Reversible route rewrite is applied when `RUNFABRIC_DEV_STREAM_IBM_OPENWHISK_SET_URL` and `RUNFABRIC_DEV_STREAM_IBM_OPENWHISK_RESTORE_URL` are configured; otherwise behavior falls back to lifecycle-only mode with explicit diagnostics.                                                                                                                                                                                                                                                                                                                                                                                           |

## Out of scope (for now)

- Streaming logs from the deployed function to the terminal (use `runfabric invoke logs`).
- Multi-function streaming (start with single function or default handler).

## Pending work TODO

- Expand deployed-environment integration coverage beyond the current env-gated AWS live-stream path.

## Gateway hook contract

For providers using conditional gateway rewrite hooks, configure provider-specific set and restore endpoints:

- `RUNFABRIC_DEV_STREAM_<PROVIDER>_SET_URL`
- `RUNFABRIC_DEV_STREAM_<PROVIDER>_RESTORE_URL`
- Optional bearer token: `RUNFABRIC_DEV_STREAM_<PROVIDER>_TOKEN`

`<PROVIDER>` is the uppercase provider name with non-alphanumeric characters replaced by `_`.

Examples:

- `azure-functions` -> `AZURE_FUNCTIONS`
- `digitalocean-functions` -> `DIGITALOCEAN_FUNCTIONS`
- `ibm-openwhisk` -> `IBM_OPENWHISK`

## Implementation notes

- **Tunnel:** Accept `--tunnel-url` from user (e.g. from ngrok or cloudflared).
- **Safety:** Banner when auto-wire runs so production is not pointed at a dev machine by mistake.
- **Rollback:** On SIGINT (Ctrl+C), the CLI restores all API Gateway routes to their original Lambda integrations and deletes the temporary HTTP_PROXY integration.

## See also

- [ROADMAP.md](../developer/ROADMAP.md) — Next steps (Dev mode).
- [TESTING_GUIDE.md](TESTING_GUIDE.md) — `call-local` and invoke patterns.
