#!/usr/bin/env bash
# Run the Go SDK test suite.
set -euo pipefail
HERE="$(cd "$(dirname "$0")/.." && pwd)"
cd "$HERE/sdks/go"
exec go test -v -count=1 ./... "$@"
