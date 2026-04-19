#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"

pid_file="${SMOKEY_STATE_DIR}/vaultlined.pid"
store_data_home="${SMOKEY_STATE_DIR}/data"
store_dir="${SMOKEY_STATE_DIR}/store"
store_config_dir="${SMOKEY_STATE_DIR}/config"
stores_dir="${SMOKEY_STATE_DIR}/stores"

pid_belongs_to_state_dir() {
  local pid="$1" cmdline=""
  [[ -n "${pid}" ]] && kill -0 "${pid}" >/dev/null 2>&1 || return 1
  if [[ -r "/proc/${pid}/cmdline" ]]; then
    cmdline="$(tr '\0' ' ' < "/proc/${pid}/cmdline" 2>/dev/null || true)"
  else
    cmdline="$(ps -p "${pid}" -o args= 2>/dev/null || true)"
  fi
  [[ "${cmdline}" == *"${SMOKEY_STATE_DIR}"* ]]
}

if [[ -f "${pid_file}" ]]; then
  pid="$(cat "${pid_file}")"
  if pid_belongs_to_state_dir "${pid}"; then
    if [[ -n "${VAULTLINE_TEST_ADDR:-}" ]]; then
      echo "[999-teardown] stopping vaultlined via CLI"
      go run ./cmd/vaultline --addr "${VAULTLINE_TEST_ADDR}" daemon-stop >/dev/null 2>&1 || true
      sleep 0.5
    fi
    kill -0 "${pid}" >/dev/null 2>&1 && { kill "${pid}" >/dev/null 2>&1 || true; wait "${pid}" >/dev/null 2>&1 || true; }
  fi
fi

echo "[999-teardown] removing test stores and config"
rm -rf "${store_dir}" "${stores_dir}" "${store_data_home}" "${store_config_dir}" >/dev/null 2>&1 || true
