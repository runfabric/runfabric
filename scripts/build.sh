#!/usr/bin/env bash
# Build runfabric CLI and run release checks.
# Usage: ./scripts/build.sh [target]
#   target: build (default) | release-check | clean

set -e
cd "$(dirname "$0")/.."

TARGET="${1:-build}"
case "$TARGET" in
  build)       make build ;;
  release-check) make release-check ;;
  clean)       make clean ;;
  *)           echo "Usage: $0 [build|release-check|clean]" >&2; exit 1 ;;
esac
