# Engine Feasibility Matrix (P8-R2 Phase 1)

This matrix captures provider readiness for `runtimeMode: engine` in planning.

`engineRuntime` classes:

- `custom-runtime`: provider can execute engine bundle via custom runtime path.
- `container`: provider can execute engine bundle via container image path.
- `unsupported`: no supported engine bundle path in current provider adapter contract.

| Provider | engineRuntime | Notes |
| --- | --- | --- |
| `aws-lambda` | `custom-runtime` | Custom runtime and container paths exist; planner treats provider as engine-capable. |
| `gcp-functions` | `custom-runtime` | Provider marked engine-capable for Phase 1 planning contract. |
| `azure-functions` | `unsupported` | Engine mode rejected by planner. |
| `kubernetes` | `container` | Engine bundle expected through container image workflow. |
| `cloudflare-workers` | `unsupported` | Engine mode rejected by planner. |
| `vercel` | `unsupported` | Engine mode rejected by planner. |
| `netlify` | `unsupported` | Engine mode rejected by planner. |
| `alibaba-fc` | `custom-runtime` | Custom runtime path available in adapter contract. |
| `digitalocean-functions` | `unsupported` | Engine mode rejected by planner. |
| `fly-machines` | `container` | Engine bundle expected through container image workflow. |
| `ibm-openwhisk` | `custom-runtime` | Custom runtime path available in adapter contract. |

Source of truth:

- Provider capability files under `packages/provider-*/src/capabilities.ts` (`engineRuntime`).
- Synced planner matrix: `packages/planner/src/capability-matrix.ts`.
- Validation tests: `tests/capability-sync.test.ts`.
