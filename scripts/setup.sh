#!/usr/bin/env bash
# One-time setup: create the Python venv and install each SDK's dev deps.
set -euo pipefail
HERE="$(cd "$(dirname "$0")/.." && pwd)"
cd "$HERE"

if [ ! -d .venv ]; then
  echo ">> creating .venv"
  python3 -m venv .venv
fi

echo ">> installing python deps"
./.venv/bin/pip install --quiet --upgrade pip
./.venv/bin/pip install --quiet pytest pytest-cov ruff
./.venv/bin/pip install --quiet -e sdks/python

if command -v npm >/dev/null 2>&1; then
  echo ">> installing typescript deps"
  (cd sdks/typescript && npm install --silent)
else
  echo "!! npm not found, skipping typescript deps"
fi

if command -v go >/dev/null 2>&1; then
  echo ">> verifying go module"
  (cd sdks/go && go vet ./... >/dev/null)
else
  echo "!! go not found, skipping go check"
fi

echo ">> done. Run: ./scripts/test-all.sh"
