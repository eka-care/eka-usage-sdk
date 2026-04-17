#!/usr/bin/env bash
# Run all three SDK test suites sequentially.
set -uo pipefail
HERE="$(cd "$(dirname "$0")/.." && pwd)"

pass=0
fail=0
failed_suites=()

run() {
  local name="$1" script="$2"
  echo ""
  echo "==========================="
  echo ">> $name"
  echo "==========================="
  if bash "$HERE/scripts/$script"; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
    failed_suites+=("$name")
  fi
}

run "python" "test-python.sh"
run "go"     "test-go.sh"
run "ts"     "test-ts.sh"

echo ""
echo "==========================="
echo ">> summary: $pass passed, $fail failed"
if [ $fail -gt 0 ]; then
  echo ">> failed: ${failed_suites[*]}"
  exit 1
fi
