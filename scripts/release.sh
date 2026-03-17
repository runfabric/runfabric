#!/usr/bin/env bash
# Release helper: run pre-release checks, tag+push, or goreleaser snapshot.
# Usage:
#   ./scripts/release.sh check    - run release-check only
#   ./scripts/release.sh tag     - create tag v$(cat VERSION) and push (triggers CI release)
#   ./scripts/release.sh snapshot - build artifacts locally (no publish)
#   ./scripts/release.sh         - show help
#
# Full release from local: update VERSION, run check, then ./scripts/release.sh tag.
# CI runs .github/workflows/release.yml (goreleaser + npm publish).

set -e
cd "$(dirname "$0")/.."

VERSION_FILE="${VERSION_FILE:-VERSION}"
VER=$(cat "$VERSION_FILE" 2>/dev/null | tr -d '\n' || true)
if [ -z "$VER" ]; then
  echo "Cannot read version from $VERSION_FILE" >&2
  exit 1
fi
TAG="v${VER}"

CMD="${1:-help}"
case "$CMD" in
  check)
    echo "Running release-check (build + test)..."
    make release-check
    echo "Release check passed."
    ;;
  tag)
    echo "Creating and pushing tag $TAG (triggers CI release)..."
    git tag "$TAG"
    git push origin "$TAG"
    echo "Pushed $TAG. CI will run goreleaser and npm publish."
    ;;
  snapshot)
    echo "Running goreleaser in snapshot mode (local artifacts only)..."
    command -v goreleaser >/dev/null 2>&1 || { echo "Install goreleaser: https://goreleaser.com/install/" >&2; exit 1; }
    goreleaser release --snapshot --clean
    echo "Artifacts in dist/"
    ;;
  help|*)
    echo "Usage: $0 check | tag | snapshot"
    echo "  check    - run make release-check (build + test)"
    echo "  tag      - git tag v\$(cat VERSION) and push (triggers CI release + npm publish)"
    echo "  snapshot - run goreleaser --snapshot (local dist/)"
    echo ""
    echo "Release from local:"
    echo "  1. Update VERSION and CHANGELOG; commit."
    echo "  2. ./scripts/release.sh check"
    echo "  3. ./scripts/release.sh tag"
    exit 0
    ;;
esac
