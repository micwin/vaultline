#!/usr/bin/env bash
set -euo pipefail

TEST_ROOT="${SMOKEY_TEST_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
if [[ "$(basename "${TEST_ROOT}")" != "tests.d" ]]; then
  TEST_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
fi
PROJECT_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
STATE_DIR="${PROJECT_ROOT}/.testrun"
ENV_FILE="${STATE_DIR}/env"
PID_FILE="${STATE_DIR}/vaultlined.pid"
LOG_FILE="${STATE_DIR}/vaultlined.log"
STORE_CONFIG_FILE="${VAULTLINE_TEST_STORE_CONFIG:-${STATE_DIR}/config/stores.json}"
STORE_CONFIG_HOME="$(dirname "${STORE_CONFIG_FILE}")"
STORE_DATA_HOME="${VAULTLINE_TEST_STORE_DATA:-${STATE_DIR}/data}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[030-workflow] state file missing (${ENV_FILE})" >&2
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
SECRET_NAME="restart.secret"
VALUE_FILE="$(mktemp "${STATE_DIR}/persist.XXXX")"
trap 'rm -f "${VALUE_FILE}"' EXIT

SECRET_VALUE="persist-$(date +%s%N)"

echo "${SECRET_VALUE}" | "${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret set --name "${SECRET_NAME}" --stdin >/dev/null

"${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret get --name "${SECRET_NAME}" --out "${VALUE_FILE}" >/dev/null
if [[ "$(cat "${VALUE_FILE}")" != "${SECRET_VALUE}" ]]; then
  echo "[030-workflow] mismatch before restart" >&2
  exit 1
fi

echo "[030-workflow] restarting daemon"
OLD_PID="${VAULTLINE_TEST_PID:-}"
if [[ -n "${OLD_PID}" ]] && kill -0 "${OLD_PID}" >/dev/null 2>&1; then
  "${VAULTLINE_CLI[@]}" --addr "${ADDR}" daemon-stop >/dev/null 2>&1 || true
  sleep 0.5
  kill "${OLD_PID}" >/dev/null 2>&1 || true
  wait "${OLD_PID}" >/dev/null 2>&1 || true
fi

VAULTLINE_PASSPHRASE="${VAULTLINE_TEST_PASS}" \
XDG_CONFIG_HOME="${STORE_CONFIG_HOME}" \
XDG_DATA_HOME="${STORE_DATA_HOME}" \
"${VAULTLINE_CLI[@]}" daemon --addr "${ADDR}" --store-dir "${VAULTLINE_TEST_STORE}" --config-file "${STORE_CONFIG_FILE}" >>"${LOG_FILE}" 2>&1 &
NEW_PID=$!
echo "${NEW_PID}" > "${PID_FILE}"
cat > "${ENV_FILE}" <<EOF_ENV
VAULTLINE_TEST_ADDR=${ADDR}
VAULTLINE_TEST_PASS=${VAULTLINE_TEST_PASS}
VAULTLINE_TEST_STORE=${VAULTLINE_TEST_STORE}
VAULTLINE_TEST_STORE_CONFIG=${STORE_CONFIG_FILE}
VAULTLINE_TEST_STORE_DATA=${STORE_DATA_HOME}
VAULTLINE_TEST_PID=${NEW_PID}
VAULTLINE_TEST_LOG=${LOG_FILE}
EOF_ENV

READY=0
for _ in $(seq 1 10); do
  if "${VAULTLINE_CLI[@]}" --addr "${ADDR}" health >/dev/null 2>&1; then
    READY=1
    break
  fi
  sleep 0.5
done
if [[ "${READY}" -ne 1 ]]; then
  echo "[030-workflow] daemon failed to restart" >&2
  exit 1
fi

"${VAULTLINE_CLI[@]}" --addr "${ADDR}" secret get --name "${SECRET_NAME}" --out "${VALUE_FILE}" >/dev/null
if [[ "$(cat "${VALUE_FILE}")" != "${SECRET_VALUE}" ]]; then
  echo "[030-workflow] mismatch after restart" >&2
  exit 1
fi

echo "[030-workflow] ok"
