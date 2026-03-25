#!/usr/bin/env bash
set -euo pipefail

TEST_ROOT="${SMOKEY_TEST_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
if [[ "$(basename "${TEST_ROOT}")" != "tests.d" ]]; then
  TEST_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
fi
PROJECT_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
STATE_DIR="${PROJECT_ROOT}/.testrun"
ENV_FILE="${STATE_DIR}/env"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[010-smoke] state file missing (${ENV_FILE})" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "${ENV_FILE}"

ADDR="${VAULTLINE_TEST_ADDR:-127.0.0.1:19428}"
if [[ -n "${VAULTLINE_TEST_BIN:-}" ]]; then
  VAULTLINE_CLI=("${VAULTLINE_TEST_BIN}")
else
  VAULTLINE_CLI=(go run ./cmd/vaultline)
fi

echo "[010-smoke] running go test"
go test ./...

TMP_VALUE="$(mktemp "${STATE_DIR}/value.XXXX")"

echo "[010-smoke] storing and reading secret through CLI"
printf "super-secret" | "${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret put --name smoke.token --stdin >/dev/null
"${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret get --name smoke.token --out "${TMP_VALUE}" >/dev/null

if [[ "$(cat "${TMP_VALUE}")" != "super-secret" ]]; then
  echo "[010-smoke] secret mismatch"
  exit 1
fi

rm -f "${TMP_VALUE}"
echo "[010-smoke] ok"
