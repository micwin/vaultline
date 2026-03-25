#!/usr/bin/env bash
set -euo pipefail

TEST_ROOT="${SMOKEY_TEST_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
if [[ "$(basename "${TEST_ROOT}")" != "tests.d" ]]; then
  TEST_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
fi
PROJECT_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
STATE_DIR="${PROJECT_ROOT}/.testrun"
STORE_DIR="${STATE_DIR}/store"
LOG_FILE="${STATE_DIR}/vaultlined.log"
PID_FILE="${STATE_DIR}/vaultlined.pid"
ENV_FILE="${STATE_DIR}/env"
BIN_DIR="${STATE_DIR}/bin"
CLI_BIN="${BIN_DIR}/vaultline"
ADDR="127.0.0.1:19428"
PASS="vaultline-smoke-pass"

echo "[000-setup] preparing workspace under ${STATE_DIR}"
if [[ -f "${STATE_DIR}/vaultlined.pid" ]]; then
  OLD_PID="$(cat "${STATE_DIR}/vaultlined.pid")"
  if [[ -n "${OLD_PID}" ]] && kill -0 "${OLD_PID}" >/dev/null 2>&1; then
    echo "[000-setup] stopping stale daemon (${OLD_PID})"
    kill "${OLD_PID}" >/dev/null 2>&1 || true
    sleep 1
  fi
fi
rm -rf "${STATE_DIR}"
mkdir -p "${STORE_DIR}" "${BIN_DIR}"

echo "[000-setup] building vaultline binary"
GO_BIN=$(command -v go)
if [[ -z "${GO_BIN}" ]]; then
  echo "[000-setup] go toolchain not found" >&2
  exit 1
fi
GOOS="" GOARCH="" go build -o "${CLI_BIN}" ./cmd/vaultline

VAULTLINE_PASSPHRASE="${PASS}" "${CLI_BIN}" daemon --addr "${ADDR}" --store-dir "${STORE_DIR}" >"${LOG_FILE}" 2>&1 &
PID=$!
echo "${PID}" > "${PID_FILE}"

echo "[000-setup] waiting for daemon (${PID})"
READY=0
for _ in $(seq 1 10); do
  if "${CLI_BIN}" --addr "${ADDR}" health >/dev/null 2>&1; then
    READY=1
    break
  fi
  sleep 0.5
done

if [[ "${READY}" -ne 1 ]]; then
  echo "[000-setup] daemon failed to start" >&2
  kill "${PID}" >/dev/null 2>&1 || true
  wait "${PID}" >/dev/null 2>&1 || true
  exit "${SMOKEY_SKIP_CODE:-20}"
fi

cat > "${ENV_FILE}" <<EOF
VAULTLINE_TEST_ADDR=${ADDR}
VAULTLINE_TEST_PASS=${PASS}
VAULTLINE_TEST_STORE=${STORE_DIR}
VAULTLINE_TEST_PID=${PID}
VAULTLINE_TEST_LOG=${LOG_FILE}
VAULTLINE_TEST_BIN=${CLI_BIN}
EOF

echo "[000-setup] vaultlined ready on ${ADDR}"
