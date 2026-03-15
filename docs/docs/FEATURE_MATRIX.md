# Feature Matrix

Mapping of user-facing capabilities to CLI commands and implementation status.

| Feature | CLI / Flow | Status |
|--------|------------|--------|
| **Deploy code to cloud provider** | `runfabric deploy` | ✅ Implemented (AWS: control plane + SDK; others: REST/SDK via `internal/deploy/api`, no CLI required) |
| **Run code locally** | `runfabric call-local --serve`, `runfabric dev` | ✅ Implemented (local HTTP server; attach debugger to process) |
| **Test the code** | `runfabric test` | ✅ Implemented (runs npm test / go test / pytest from project) |
| **Debug the code** | `runfabric debug` | ✅ Implemented (same as call-local --serve, prints PID for debugger attach) |
| **Monitor the code** | `runfabric logs`, `runfabric traces`, `runfabric metrics` | ✅ Implemented (logs: real API for Fly, Vercel, Netlify, DO, Cloudflare, IBM; invoke: real HTTP or OpenWhisk API; no stub) |
| **Scale the code** | Serverless auto-scales | ✅ Documented: set `memory`/`timeout` and provider concurrency in `runfabric.yml`; no separate scale command |
| **Rollback the code** | `runfabric recover --mode rollback` | ✅ Implemented (recover from failed deploy; AWS rollback in provider) |
| **Delete the code** | `runfabric remove` | ✅ Implemented (AWS: control plane; API providers: `internal/deploy/api.Remove` deletes via provider API and clears receipt) |
| **Create the code** | `runfabric init` | ✅ Implemented (scaffold project; validation + flags; file generation TODO) |
| **Update the code** | `runfabric deploy` (re-deploy) | ✅ Implemented (deploy updates existing deployment) |
| **List the code** | `runfabric list` | ✅ Implemented (lists functions from config + deployment status from receipt) |

## Command reference

- **deploy** – Deploy to cloud (AWS: control plane + SDK; other providers: REST/SDK in `internal/deploy/api`, no provider CLI required).
- **remove** – Remove deployed resources (API providers: delete via provider API, then clear `.runfabric/<stage>.json` receipt).
- **call-local** – Run locally; use `--serve` to start HTTP server.
- **dev** – Same as call-local with server (local testing).
- **test** – Run project tests (npm test / go test / pytest).
- **debug** – Run locally and print PID for debugger attach.
- **logs** – Stream function logs (provider-specific).
- **traces** – Trace info (receipt + message; use provider console for full traces).
- **metrics** – Metrics (receipt + message; use provider console for full metrics).
- **recover** – Rollback or resume after failed deploy.
- **remove** – Delete deployed resources (real delete for AWS and all `internal/deploy/api` providers).
- **init** – Create new project.
- **list** – List functions and deployment status.
