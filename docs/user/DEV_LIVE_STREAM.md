# Dev mode: live stream of remote invocations to local

**Status:** Implemented. Local server + instructions; **auto-wire** for AWS (API Gateway HTTP API) is implemented: when both `--stream-from` and `--tunnel-url` are set, the CLI points all API Gateway routes at the tunnel and restores them on exit (Ctrl+C). Complements existing `runfabric dev` presets (http, queue, storage, cron, etc.).

## Quick navigation

- **Just run local server**: `runfabric dev --stream-from <stage>`
- **AWS auto-wire to your tunnel**: add `--tunnel-url <https://...>`
- **Provider capabilities**: see the provider-specific table below

## Goal

Route invocations that would go to the **deployed** function (e.g. on AWS Lambda) to a **local** process instead (e.g. `runfabric call-local --serve`), so you can debug with real events without deploying every change.

## Proposed UX

```bash
# Terminal 1: start local receiver (serves HTTP and optionally registers with provider)
runfabric dev --stream-from dev

# Optional: use a tunnel URL (e.g. ngrok, cloudflared) so the provider can reach your machine
runfabric dev --stream-from dev --tunnel-url https://abc.ngrok.io
```

- `--stream-from <stage>`: Stage to “stream” from; invocations targeting that stage are forwarded to the local process.
- When supported by the provider, the CLI may configure the provider to send events to the tunnel URL (e.g. Lambda function URL → tunnel, or EventBridge rule → webhook).

## Provider-specific behavior

| Provider         | Approach |
|------------------|----------|
| AWS Lambda       | **Implemented:** When `--stream-from` and `--tunnel-url` are set, the CLI finds the HTTP API for the stage, creates an HTTP_PROXY integration to the tunnel URL, and points all routes at it; on exit (SIGINT), routes are restored and the temporary integration is deleted. Function-URL-only deployments (no HTTP API) are not auto-wired; deploy with HTTP events first. |
| GCP Cloud Functions | **Local server only:** `runfabric dev --stream-from <stage>` runs the local server; auto-wire is not implemented. Use [Firebase Local Emulator](https://firebase.google.com/docs/functions/local-emulator) or point Eventarc/Cloud Scheduler to your tunnel URL manually. |
| Cloudflare Workers | **Local server only:** Run `runfabric dev --stream-from <stage>` for the local server; use `wrangler dev` for full Workers dev experience, or point your worker to the tunnel URL manually. |
| Others            | Local server runs; point the provider’s invocation target at your tunnel URL manually where supported. |

## Out of scope (for now)

- Streaming logs from the deployed function to the terminal (use `runfabric logs`).
- Multi-function streaming (start with single function or default handler).

## Implementation notes

- **Tunnel:** Accept `--tunnel-url` from user (e.g. from ngrok or cloudflared).
- **Safety:** Banner when auto-wire runs so production is not pointed at a dev machine by mistake.
- **Rollback:** On SIGINT (Ctrl+C), the CLI restores all API Gateway routes to their original Lambda integrations and deletes the temporary HTTP_PROXY integration.

## See also

- [ROADMAP.md](ROADMAP.md) — Next steps (Dev mode).
- [TESTING_GUIDE.md](TESTING_GUIDE.md) — `call-local` and invoke patterns.
