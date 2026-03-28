# Build and release

## Quick navigation

- **Most common tasks**: Quick reference table
- **Building locally**: Go CLI build
- **Releasing**: Release from local (tag + push)

## Quick reference

| Task                                 | Command                                             |
| ------------------------------------ | --------------------------------------------------- |
| Build local binaries                 | `make build`                                        |
| Run tests                            | `make test`                                         |
| Pre-release check (build + test)     | `make release-check`                                |
| Show version                         | `make version` or `./bin/runfabric -v`              |
| Local release artifacts (no publish) | `goreleaser release --snapshot --clean`             |
| Cut a release (from local)           | See [Release from local](#release-from-local) below |

## Version

- **VERSION** (root) is the single source for the CLI version when building locally.
- For released builds, the version comes from the git tag (e.g. `v0.2.0`). GoReleaser injects it via `-ldflags` when the release workflow runs.
- Bump **VERSION** and **CHANGELOG.md** before tagging. See **VERSIONING.md** and **RELEASE_PROCESS.md**.

## Go CLI build

```bash
make build          # → bin/runfabric, bin/runfabricd, and bin/runfabricw (version from VERSION file)
make release-check  # build + go build ./... + go test ./...
make clean          # remove bin/ and go caches
```

The binaries are built with `-trimpath` and ldflags for `platform/core/model.Version`.

Binary intent:

- `runfabric`: control-plane CLI (doctor/plan/build/deploy/remove/invoke/state/extensions/router/admin/project/config + workflow).
- `runfabricw`: workload-plane CLI (workflow runtime commands only: run/status/cancel/replay).
- `runfabricd`: daemon-plane CLI (`runfabricd`, `runfabricd start|stop|restart|status`) for long-running config API/dashboard operations.
- Code boundary: `runfabric` root is built from `internal/cli`; `runfabricd` root is built from `internal/cli/daemon`; `runfabricw` root is built from `internal/cli/worker`.

## SDKs

- **packages/node/cli** – `@runfabric/cli` (Node CLI wrapper). Published releases include platform-specific `runfabric-<os-arch>` binaries in the package (`bin/`). Resolution order: package `bin/` → repo root `bin/` → `~/.runfabric/bin/`.
- **packages/node/sdk** – `@runfabric/sdk` (Node SDK: handler contract, HTTP adapter, framework adapters). No binaries.

## Release from local

Releases are triggered by **pushing a version tag**; CI then builds artifacts and publishes.

### 1. Pre-release

- Set the version in **VERSION** (e.g. `0.2.0` or `0.2.0-beta.0`).
- Update **CHANGELOG.md** with a `## [<version>]` section. Optionally add **release-notes/<version>.md** (see RELEASE_PROCESS.md for signing).
- Commit everything.
- Run the release gate:

  ```bash
  make release-check
  ```

### 2. Tag and push (triggers CI release)

From repo root:

```bash
make release-tag
```

### 3. What happens in CI

On push of a tag matching `v*`, **.github/workflows/release.yml** runs:

- **goreleaser**: builds all platforms for `runfabric`, `runfabricd`, and `runfabricw`, then creates GitHub Release tarballs/zips and checksums.
- **npm**: builds all platform binaries, copies platform-specific `runfabric-<os-arch>` binaries into `packages/node/cli/bin/`, sets package version from the tag, and publishes **@runfabric/cli** and **@runfabric/sdk**. Requires **NPM_TOKEN** in repo secrets.

### 4. Optional: local snapshot (no publish)

To build release-style artifacts locally without publishing:

```bash
goreleaser release --snapshot --clean   # outputs in dist/
```

To build all platform binaries and pack the npm package locally (for testing):

```bash
make build-all-platforms
mkdir -p packages/node/cli/bin && (for f in bin/runfabric-*; do [ -f "$f" ] && cp "$f" packages/node/cli/bin/; done) && [ -n "$(ls -A packages/node/cli/bin 2>/dev/null)" ] || cp bin/runfabric packages/node/cli/bin/
cd packages/node/cli && npm pack
cd ../sdk && npm pack
```

Note: `runfabricd` and `runfabricw` binaries are built for daemon/worker usage and GitHub release artifacts, but they are not packaged into the Node CLI wrapper.

## Release (CI) summary

1. Update **VERSION**, **CHANGELOG.md**, and **release-notes/<version>.md**.
2. Commit, then run `make release-check`.
3. Run `make release-tag`.
4. CI runs **.github/workflows/release.yml** (goreleaser + npm publish). Ensure **NPM_TOKEN** is set in repo secrets for npm publish.

See **RELEASE_PROCESS.md** for release notes signing and full steps.
