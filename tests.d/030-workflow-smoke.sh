#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"
vaultline_require_setup

pid_file="${SMOKEY_STATE_DIR}/vaultlined.pid"
log_file="${SMOKEY_STATE_DIR}/vaultlined.log"
value_file="$(mktemp "${SMOKEY_STATE_DIR}/persist.XXXX")"
trap 'rm -f "${value_file}"' EXIT

secret_name="restart.secret"
secret_value="persist-$(date +%s%N)"

printf "%s" "${secret_value}" | vaultline_run secret set --name "${secret_name}" --stdin >/dev/null
vaultline_run secret get --name "${secret_name}" --out "${value_file}" >/dev/null
[[ "$(cat "${value_file}")" == "${secret_value}" ]] || { echo "[030-workflow] mismatch before restart" >&2; exit 1; }

echo "[030-workflow] restarting daemon"
if [[ -n "${VAULTLINE_TEST_PID}" ]] && kill -0 "${VAULTLINE_TEST_PID}" >/dev/null 2>&1; then
  vaultline_run daemon-stop >/dev/null 2>&1 || true
  sleep 0.5
  kill "${VAULTLINE_TEST_PID}" >/dev/null 2>&1 || true
  wait "${VAULTLINE_TEST_PID}" >/dev/null 2>&1 || true
fi

VAULTLINE_PASSPHRASE="${VAULTLINE_TEST_PASS}" \
XDG_CONFIG_HOME="$(dirname "${VAULTLINE_TEST_STORE_CONFIG}")" \
XDG_DATA_HOME="${VAULTLINE_TEST_STORE_DATA}" \
go run ./cmd/vaultline daemon --addr "${VAULTLINE_TEST_ADDR}" --store-dir "${VAULTLINE_TEST_STORE}" --config-file "${VAULTLINE_TEST_STORE_CONFIG}" >>"${log_file}" 2>&1 &
VAULTLINE_TEST_PID=$!
echo "${VAULTLINE_TEST_PID}" > "${pid_file}"

smokey_env_save VAULTLINE_TEST_PID
smokey_env_save VAULTLINE_TEST_LOG

for _ in $(seq 1 10); do
  vaultline_run health >/dev/null 2>&1 && break
  sleep 0.5
done
vaultline_run health >/dev/null 2>&1 || { echo "[030-workflow] daemon failed to restart" >&2; exit 1; }

vaultline_run secret get --name "${secret_name}" --out "${value_file}" >/dev/null
[[ "$(cat "${value_file}")" == "${secret_value}" ]] || { echo "[030-workflow] mismatch after restart" >&2; exit 1; }

echo "[030-workflow] ok"
