# Build and release

## Quick reference

| Task | Command |
|------|---------|
| Build CLI binary | `make build` or `npm run build` |
| Run tests | `make test` or `npm run test` |
| Pre-release check (build + test) | `make release-check` or `npm run release:check` |
| Show version | `make version` or `./bin/runfabric -v` |
| Local release artifacts (no publish) | `./scripts/release.sh snapshot` |
| Cut a release | Tag `v<VERSION>` and push; CI runs goreleaser |

## Version

- **VERSION** (root) is the single source for the CLI version when building locally.
- For released builds, the version comes from the git tag (e.g. `v0.2.0`). GoReleaser injects it via `-ldflags` when the release workflow runs.
- Bump **VERSION** and **CHANGELOG.md** before tagging. See **VERSIONING.md** and **RELEASE_PROCESS.md**.

## Go CLI build

```bash
make build          # → bin/runfabric (version from VERSION file)
make release-check  # build + go build ./... + go test ./...
make clean          # remove bin/ and go caches
```

The binary is built with `-trimpath` and ldflags for `runtime.Version` and `runtime.ProtocolVersion`.

## SDKs

- **sdk/node** – `@runfabric/cli` (Node wrapper; `postinstall` can fetch the CLI binary). Publish with `npm publish` from that directory after bumping version.
- **sdk/ts** – `@runfabric/sdk` (TypeScript). Run `npm run build` before publish; version in `package.json`.

## Release (CI)

1. Update **VERSION**, **CHANGELOG.md**, and **release-notes/<version>.md**.
2. Commit, then create and push a tag:  
   `git tag v$(cat VERSION) && git push origin v$(cat VERSION)`
3. **.github/workflows/release.yml** runs on tag push `v*`, runs GoReleaser, and creates the GitHub release with artifacts (tarballs/zips per OS/arch and checksums).

See **RELEASE_PROCESS.md** for full steps (release notes signing, npm publish, etc.).
