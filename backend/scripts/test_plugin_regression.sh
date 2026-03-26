#!/usr/bin/env sh
set -eu
set -o pipefail 2>/dev/null || true

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "${ROOT_DIR}"

TEST_PATTERN='Plugin|plugin|Hook|hook|Bootstrap|Extensions|JS|js|Upload'
# Let Go resolve the current package graph so directory refactors do not break CI.
PACKAGES="./internal/..."

echo "[plugin-regression] running plugin baseline tests..."
echo "[plugin-regression] pattern: ${TEST_PATTERN}"
echo "[plugin-regression] packages: ${PACKAGES}"

go test ${PACKAGES} -run "${TEST_PATTERN}" -count=1

echo "[plugin-regression] done."
