#!/usr/bin/env bash
set -euo pipefail

TEST_ROOT="${SMOKEY_TEST_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
if [[ "$(basename "${TEST_ROOT}")" != "tests.d" ]]; then
  TEST_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
fi
PROJECT_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
STATE_DIR="${PROJECT_ROOT}/.testrun"
PID_FILE="${STATE_DIR}/vaultlined.pid"
ENV_FILE="${STATE_DIR}/env"
STORE_DATA_HOME="${STATE_DIR}/data"
STORE_DIR="${STATE_DIR}/store"
STORE_CONFIG_DIR="${STATE_DIR}/config"
STORES_DIR="${STATE_DIR}/stores"

VAULTLINE_CLI=(go run ./cmd/vaultline)
VAULTLINE_ADDR="127.0.0.1:8428"
CAN_USE_DAEMON_STOP=false
if [[ -f "${ENV_FILE}" ]]; then
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  if [[ -n "${VAULTLINE_TEST_BIN:-}" ]]; then
    VAULTLINE_CLI=("${VAULTLINE_TEST_BIN}")
  fi
  if [[ -n "${VAULTLINE_TEST_ADDR:-}" ]]; then
    VAULTLINE_ADDR="${VAULTLINE_TEST_ADDR}"
    CAN_USE_DAEMON_STOP=true
  fi
fi

pid_belongs_to_testrun() {
  local pid="$1"
  if [[ -z "${pid}" ]] || ! kill -0 "${pid}" >/dev/null 2>&1; then
    return 1
  fi
  local cmdline=""
  if [[ -r "/proc/${pid}/cmdline" ]]; then
    cmdline="$(tr '\0' ' ' < "/proc/${pid}/cmdline" 2>/dev/null || true)"
  else
    cmdline="$(ps -p "${pid}" -o args= 2>/dev/null || true)"
  fi
  [[ "${cmdline}" == *".testrun"* ]]
}

if [[ -f "${PID_FILE}" ]]; then
  PID="$(cat "${PID_FILE}")"
  if pid_belongs_to_testrun "${PID}"; then
    if [[ "${CAN_USE_DAEMON_STOP}" == "true" ]]; then
      echo "[999-teardown] stopping vaultlined via CLI"
      if "${VAULTLINE_CLI[@]}" --addr "${VAULTLINE_ADDR}" daemon-stop >/dev/null 2>&1; then
        sleep 0.5
      fi
    fi
    if kill -0 "${PID}" >/dev/null 2>&1; then
      kill "${PID}" >/dev/null 2>&1 || true
      wait "${PID}" >/dev/null 2>&1 || true
    fi
  fi
fi

echo "[999-teardown] removing test stores and config"
rm -rf "${STORE_DIR}" "${STORES_DIR}" "${STORE_DATA_HOME}" "${STORE_CONFIG_DIR}" >/dev/null 2>&1 || true

echo "[999-teardown] removing ${STATE_DIR}"
rm -rf "${STATE_DIR}" >/dev/null 2>&1 || true
