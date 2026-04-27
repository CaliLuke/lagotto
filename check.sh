#!/usr/bin/env bash
# check.sh — run every quality gate for lagotto. Treat any failure as
# blocking. Run before pushing, before tagging a release, or whenever
# a sanity check is wanted.
#
# Stages, in order (each is independent; we run them all before
# reporting so the developer sees the full picture):
#   1. go build         — code compiles
#   2. go vet           — basic correctness lints
#   3. golangci-lint    — strict style + correctness gates
#   4. go test          — unit tests with the race detector
#   5. lagotto on self  — eat our own dogfood; zero findings expected

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

PASS=0
FAIL=0
report() {
  local name="$1"; local rc="$2"
  if [ "$rc" -eq 0 ]; then
    printf '  \033[32m✓\033[0m  %s\n' "$name"
    PASS=$((PASS + 1))
  else
    printf '  \033[31m✗\033[0m  %s (exit %d)\n' "$name" "$rc"
    FAIL=$((FAIL + 1))
  fi
}

printf '\n--- build ---\n'
go build ./...
report "go build" $?

printf '\n--- vet ---\n'
go vet ./...
report "go vet" $?

printf '\n--- lint ---\n'
if command -v golangci-lint >/dev/null 2>&1; then
  golangci-lint run ./...
  report "golangci-lint" $?
else
  printf '  \033[33m!\033[0m  golangci-lint not installed (skipping)\n'
fi

printf '\n--- test ---\n'
go test -race -count=1 ./...
report "go test -race" $?

printf '\n--- self-audit ---\n'
go build -o /tmp/lagotto-selfcheck . && \
  /tmp/lagotto-selfcheck audit --format=text . | tee /tmp/lagotto-selfcheck.out && \
  ! grep -q "^\[" /tmp/lagotto-selfcheck.out
report "lagotto on lagotto" $?
rm -f /tmp/lagotto-selfcheck /tmp/lagotto-selfcheck.out

printf '\n=========================================\n'
printf '  %d passed  %d failed\n' "$PASS" "$FAIL"
printf '=========================================\n\n'

exit $FAIL
