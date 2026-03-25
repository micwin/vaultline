#!/usr/bin/env bash
set -euo pipefail

TEST_ROOT="${SMOKEY_TEST_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}" )" && pwd)}"
if [[ "$(basename "${TEST_ROOT}")" != "tests.d" ]]; then
  TEST_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
fi
PROJECT_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
STATE_DIR="${PROJECT_ROOT}/.testrun"
ENV_FILE="${STATE_DIR}/env"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[050-secrets-list] missing env file" >&2
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

KEY_NAME="list.fire-test"
VALUE="embers-${RANDOM}"

echo "${VALUE}" | "${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret put --name "${KEY_NAME}" --stdin >/dev/null

OUTPUT="$("${VAULTLINE_CLI[@]}" --addr "${ADDR}" secrets list)"
if ! grep -q "${KEY_NAME}" <<<"${OUTPUT}"; then
  echo "[050-secrets-list] ${KEY_NAME} missing in output" >&2
  exit 1
fi

echo "[050-secrets-list] ok"
