# Framework E2E tests

Black-box end-to-end tests that drive the **real binaries** — the `runfabric`
CLI as a subprocess and the `runfabricd` daemon over its HTTP API — rather than
calling `app.*` in-process (which is what `platform/test/integration` does).
`TestMain` builds `runfabricd`, `runfabric`, and `runfabricw` once into a temp
dir; each test spawns what it needs on a free loopback port and cleans up.

These are **framework-level** tests only — they cover the CLI/daemon surface and
are provider-agnostic. Provider-specific cloud tests (deploy → invoke → logs →
remove, Step Functions, rollback, …) live **with their provider** so they travel
with it if the provider is extracted to its own repository — see
`extensions/providers/aws-lambda/floci_test.go` (build tag `floci`).

All files here are behind the `e2e` build tag, so `go test ./...` never runs
them.

## Running

```bash
make e2e                                  # no cloud, no creds
go test -tags e2e ./platform/test/e2e/... -v
```

## Lanes

| Lane | File |
| --- | --- |
| Daemon workflows | `daemon_workflow_test.go` |
| Daemon invariants | `daemon_invariants_test.go` |
| CLI black-box | `cli_test.go` |

Coverage: daemon `/healthz` `/readyz` `/version`, `validate`/`resolve`, the
durable workflow lifecycle (`run` → pause → `approve` → resume, `cancel`,
`runs`), and the cross-cutting invariants — `X-Request-Id` echo, inbound
`traceparent` join (`X-Trace-Id`), config-path confinement (traversal + absolute
paths rejected), non-loopback bind refusal without `--api-key`, and `--api-key`
enforcement. On the CLI side: `--version`, `doctor`, `init` scaffolding, and the
`workflow run`/`approve` lifecycle.

## Provider cloud tests

Providers are tested against [Floci](https://floci.io) emulators directly through
their contract methods (no CLI/daemon binaries), each in a `floci_test.go` behind
the `floci` build tag so they travel with the provider if it is extracted. What
each covers is bounded by what its emulator supports:

| Provider | Emulator | Lifecycle covered |
| --- | --- | --- |
| `aws-lambda` | `floci/floci` (4566) | Deploy → Invoke → Logs → FetchMetrics → Step Functions → rollback → Remove (real Lambda runtime) |
| `gcp-functions` | `floci/floci-gcp` (4588) | GCS source upload → Deploy → Remove (Cloud Functions **control plane only** — floci-gcp has no runtime, so no invoke/logs/metrics) |
| `azure-functions` | `floci/floci-az` (4577) | Deploy → Remove (ARM `Microsoft.Web/sites` **control plane**) |

The Azure provider's real code-push (Kudu zip deploy) + invoke is validated
separately by `TestDeployPushesCodeAndInvokeReturnsPayload` (a normal unit test
against an ARM+Kudu+function httptest double) because floci-az does not implement
Kudu and its runtime image has no arm64 manifest.

```bash
make test-floci             # all providers, RUNFABRIC_FLOCI_DOCKER=1 starts a container per test
make test-floci-aws         # or one cloud at a time
make test-floci-gcp
make test-floci-az
# or point the matching endpoint at a running emulator:
AWS_ENDPOINT_URL=http://localhost:4566   go test -tags floci ./extensions/providers/aws-lambda/...   -run Floci -v
GCP_ENDPOINT_URL=http://localhost:4588   go test -tags floci ./extensions/providers/gcp-functions/... -run Floci -v
AZURE_ENDPOINT_URL=http://localhost:4577 go test -tags floci ./extensions/providers/azure-functions/... -run Floci -v
```
