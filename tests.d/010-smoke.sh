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

echo "[010-smoke] running go test"
go test ./...

TMP_VALUE="$(mktemp "${STATE_DIR}/value.XXXX")"

echo "[010-smoke] storing and reading secret through CLI"
printf "super-secret" | go run ./cmd/vaultline --addr "${ADDR}" secret put --space default --namespace smoke --name token --stdin >/dev/null
go run ./cmd/vaultline --addr "${ADDR}" secret get --space default --namespace smoke --name token --out "${TMP_VALUE}" >/dev/null

if [[ "$(cat "${TMP_VALUE}")" != "super-secret" ]]; then
  echo "[010-smoke] secret mismatch"
  exit 1
fi

rm -f "${TMP_VALUE}"
echo "[010-smoke] ok"
