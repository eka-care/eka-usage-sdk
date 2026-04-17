#!/usr/bin/env bash
# Run the Python SDK test suite.
set -euo pipefail
HERE="$(cd "$(dirname "$0")/.." && pwd)"
cd "$HERE/sdks/python"
exec "$HERE/.venv/bin/python" -m pytest -v "$@"
