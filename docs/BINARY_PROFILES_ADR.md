# Binary Profiles ADR

Status: Accepted

## Purpose

Freeze the command ownership contract for the three Go binaries so command surfaces do not drift and automation remains predictable.

## Decision

RunFabric has three distinct binaries with strict command ownership:

- `runfabric`: control-plane operator CLI.
- `runfabricd`: daemon/service-process CLI.
- `runfabricw`: workload runtime CLI.

## Owned surfaces

- `runfabric`
  - Owns full control-plane lifecycle and operator commands.
  - Must not expose daemon lifecycle commands.
- `runfabricd`
  - Owns daemon runtime and daemon lifecycle commands: `runfabricd`, `runfabricd start|stop|restart|status`.
  - Must not expose deploy/remove/state/router or workflow runtime command groups.
- `runfabricw`
  - Owns workflow runtime commands only (`workflow run|status|cancel|replay`).
  - Must not expose control-plane command groups.

## Compatibility stance

- No backward-compatibility alias commands across binary boundaries.
- Specifically, `runfabric daemon ...` is not supported.
- Cross-binary misuse should fail clearly with command ownership guidance.

## Enforcement

- `go test ./cmd/... ./internal/cli/...`
- `make check-binary-surfaces`
- `make check-syntax`
- `make release-check`

## Migration note

Automation must invoke the owning binary directly:

- daemon operations -> `runfabricd`
- control-plane operations -> `runfabric`
- workflow runtime operations -> `runfabricw`
