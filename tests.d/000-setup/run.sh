#!/usr/bin/env bash
set -euo pipefail

export VAULTLINE_TEST_STORE="${SMOKEY_STATE_DIR}/store"
export VAULTLINE_TEST_PROJECT_STORE="${SMOKEY_STATE_DIR}/stores/project-a"
STORE_CONFIG_DIR="${SMOKEY_STATE_DIR}/config"
export VAULTLINE_TEST_STORE_CONFIG="${STORE_CONFIG_DIR}/stores.json"
export VAULTLINE_TEST_STORE_DATA="${SMOKEY_STATE_DIR}/data"
export VAULTLINE_TEST_LOG="${SMOKEY_STATE_DIR}/vaultlined.log"
PID_FILE="${SMOKEY_STATE_DIR}/vaultlined.pid"
export VAULTLINE_TEST_ADDR="127.0.0.1:19428"
export VAULTLINE_TEST_PASS="vaultline-smoke-pass"
export VAULTLINE_TEST_PID=""

echo "[000-setup] preparing workspace under ${SMOKEY_STATE_DIR}"
mkdir -p "${VAULTLINE_TEST_STORE}"

echo "[000-setup] checking go toolchain"
GO_BIN=$(command -v go)
if [[ -z "${GO_BIN}" ]]; then
  echo "[000-setup] go toolchain not found" >&2
  exit 1
fi

VAULTLINE_PASSPHRASE="${VAULTLINE_TEST_PASS}" XDG_CONFIG_HOME="${STORE_CONFIG_DIR}" XDG_DATA_HOME="${VAULTLINE_TEST_STORE_DATA}" go run ./cmd/vaultline daemon --addr "${VAULTLINE_TEST_ADDR}" --store-dir "${VAULTLINE_TEST_STORE}" --config-file "${VAULTLINE_TEST_STORE_CONFIG}" >"${VAULTLINE_TEST_LOG}" 2>&1 &
VAULTLINE_TEST_PID=$!
echo "${VAULTLINE_TEST_PID}" > "${PID_FILE}"

echo "[000-setup] waiting for daemon (${VAULTLINE_TEST_PID})"
READY=0
for _ in $(seq 1 10); do
  if go run ./cmd/vaultline --addr "${VAULTLINE_TEST_ADDR}" health >/dev/null 2>&1; then
    READY=1
    break
  fi
  sleep 0.5
done

if [[ "${READY}" -ne 1 ]]; then
  echo "[000-setup] daemon failed to start" >&2
  kill "${VAULTLINE_TEST_PID}" >/dev/null 2>&1 || true
  wait "${VAULTLINE_TEST_PID}" >/dev/null 2>&1 || true
  exit "${SMOKEY_SKIP_CODE:-20}"
fi

for name in \
  VAULTLINE_TEST_ADDR \
  VAULTLINE_TEST_PASS \
  VAULTLINE_TEST_STORE \
  VAULTLINE_TEST_PROJECT_STORE \
  VAULTLINE_TEST_STORE_CONFIG \
  VAULTLINE_TEST_STORE_DATA \
  VAULTLINE_TEST_PID \
  VAULTLINE_TEST_LOG; do
  smokey_env_save "${name}"
done

echo "[000-setup] vaultlined ready on ${VAULTLINE_TEST_ADDR}"
