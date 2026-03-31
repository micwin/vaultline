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
  echo "[040-key-validation] missing env file" >&2
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
VALUE_FILE="${STATE_DIR}/kv-check.txt"
trap 'rm -f "${VALUE_FILE}"' EXIT

SECRET_DATA="${PROJECT_ROOT}/testdata/secret-value.txt"
if [[ ! -f "${SECRET_DATA}" ]]; then
  echo "[040-key-validation] missing fixture ${SECRET_DATA}" >&2
  exit 1
fi

VALID_NAME="mötor-1.gamma@local"
INVALID_NAME="Uppercase"
INVALID_SLASH_NAME="slash/name"

cat "${SECRET_DATA}" | "${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret set --name "${VALID_NAME}" --stdin >/dev/null

"${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret get --name "${VALID_NAME}" --out "${VALUE_FILE}" >/dev/null
if ! cmp -s "${VALUE_FILE}" "${SECRET_DATA}"; then
  echo "[040-key-validation] retrieved secret mismatch" >&2
  exit 1
fi

if "${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret set --name "${INVALID_NAME}" --value bogus >/dev/null 2>&1; then
  echo "[040-key-validation] invalid name unexpectedly succeeded" >&2
  exit 1
fi

if "${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret set --name "${INVALID_SLASH_NAME}" --value bogus >/dev/null 2>&1; then
  echo "[040-key-validation] slash name unexpectedly succeeded" >&2
  exit 1
fi

echo "[040-key-validation] ok"
