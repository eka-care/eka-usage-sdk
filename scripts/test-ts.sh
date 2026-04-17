#!/usr/bin/env bash
# Run the TypeScript SDK test suite.
# Installs node_modules on first run.
set -euo pipefail
HERE="$(cd "$(dirname "$0")/.." && pwd)"
cd "$HERE/sdks/typescript"

if [ ! -d node_modules ]; then
  echo ">> installing node_modules (first run)"
  npm install --silent
fi

exec npm test --silent -- "$@"
