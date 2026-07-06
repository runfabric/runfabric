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

The `aws-lambda` provider is tested against a [Floci](https://floci.io) emulator
directly through its `Provider{}` contract (no CLI/daemon binaries), in
`extensions/providers/aws-lambda/floci_test.go`:

```bash
make test-floci        # RUNFABRIC_FLOCI_DOCKER=1 starts a Floci container per test
# or point at a running emulator:
AWS_ENDPOINT_URL=http://localhost:4566 go test -tags floci ./extensions/providers/aws-lambda/... -run Floci -v
```
