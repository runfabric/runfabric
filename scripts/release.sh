#!/usr/bin/env bash
# Release helper: run pre-release checks and optionally run goreleaser (snapshot or release).
# Usage:
#   ./scripts/release.sh check          - run release-check only
#   ./scripts/release.sh snapshot       - build artifacts locally (no publish)
#   ./scripts/release.sh                - remind to tag and push (CI runs goreleaser on v*)
#
# Full release: tag and push (e.g. git tag v0.2.0 && git push origin v0.2.0).
# GitHub Actions will run .github/workflows/release.yml and call goreleaser.

set -e
cd "$(dirname "$0")/.."

CMD="${1:-help}"
case "$CMD" in
  check)
    echo "Running release-check (build + test)..."
    make release-check
    echo "Release check passed."
    ;;
  snapshot)
    echo "Running goreleaser in snapshot mode (local artifacts only)..."
    command -v goreleaser >/dev/null 2>&1 || { echo "Install goreleaser: https://goreleaser.com/install/" >&2; exit 1; }
    goreleaser release --snapshot --clean
    echo "Artifacts in dist/"
    ;;
  help|*)
    echo "Usage: $0 check | snapshot"
    echo "  check    - run make release-check (build + test)"
    echo "  snapshot - run goreleaser --snapshot (local dist/)"
    echo ""
    echo "To cut a release: update VERSION and CHANGELOG, then:"
    echo "  git tag v\$(cat VERSION) && git push origin v\$(cat VERSION)"
    echo "CI will run goreleaser and create the GitHub release."
    exit 0
    ;;
esac
