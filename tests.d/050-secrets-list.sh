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
PROJECT_STORE_DIR="${VAULTLINE_TEST_PROJECT_STORE:-${STATE_DIR}/stores/project-a}"
STORE_CONFIG_FILE="${VAULTLINE_TEST_STORE_CONFIG:-${STATE_DIR}/config/stores.json}"
if [[ -n "${VAULTLINE_TEST_BIN:-}" ]]; then
  VAULTLINE_CLI=("${VAULTLINE_TEST_BIN}")
else
  VAULTLINE_CLI=(go run ./cmd/vaultline)
fi

KEY_NAME="list.fire-test"
VALUE="embers-${RANDOM}"
PROJECT_KEY_NAME="list.project-test"
PROJECT_VALUE="project-${RANDOM}"

echo "${VALUE}" | "${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret set "local:${KEY_NAME}" --stdin >/dev/null
XDG_CONFIG_HOME="$(dirname "${STORE_CONFIG_FILE}")" "${VAULTLINE_CLI[@]}" --addr "${ADDR}" store init project-a "${PROJECT_STORE_DIR}" >/dev/null 2>&1 || true
XDG_CONFIG_HOME="$(dirname "${STORE_CONFIG_FILE}")" "${VAULTLINE_CLI[@]}" --addr "${ADDR}" store unseal project-a >/dev/null 2>&1 || true
echo "${PROJECT_VALUE}" | "${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret set "project-a:${PROJECT_KEY_NAME}" --stdin >/dev/null

OUTPUT_LOCAL="$("${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret list local:)"
if ! grep -q "local:${KEY_NAME}" <<<"${OUTPUT_LOCAL}"; then
  echo "[050-secrets-list] local:${KEY_NAME} missing in output" >&2
  exit 1
fi
if grep -q "project-a:${PROJECT_KEY_NAME}" <<<"${OUTPUT_LOCAL}"; then
  echo "[050-secrets-list] project key leaked into local list" >&2
  exit 1
fi

OUTPUT_PROJECT="$("${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret list project-a:)"
if ! grep -q "project-a:${PROJECT_KEY_NAME}" <<<"${OUTPUT_PROJECT}"; then
  echo "[050-secrets-list] project-a:${PROJECT_KEY_NAME} missing in output" >&2
  exit 1
fi

echo "[050-secrets-list] ok"
