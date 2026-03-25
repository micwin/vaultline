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
  echo "[020-api-health] state file missing (${ENV_FILE})" >&2
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

echo "[020-api-health] checking CLI health output"
CLI_OUTPUT="$("${VAULTLINE_CLI[@]}" --addr "${ADDR}" health)"
echo "[020-api-health] cli: ${CLI_OUTPUT}"

echo "[020-api-health] querying REST health endpoint"
HTTP_OUTPUT="$(curl -fsS "http://${ADDR}/api/v1/health")"
echo "[020-api-health] rest: ${HTTP_OUTPUT}"

if ! grep -q '"sealed":false' <<<"${HTTP_OUTPUT}"; then
  echo "[020-api-health] daemon unexpectedly sealed" >&2
  exit 1
fi

echo "[020-api-health] ok"
