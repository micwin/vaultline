#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"
vaultline_require_setup

echo "[010-smoke] running go test"
go test ./...

tmp_value="$(mktemp "${SMOKEY_STATE_DIR}/value.XXXX")"
trap 'rm -f "${tmp_value}"' EXIT

echo "[010-smoke] storing and reading secret through CLI"
printf "super-secret" | vaultline_run secret set default:smoke.token --stdin >/dev/null
vaultline_run secret get default:smoke.token --out "${tmp_value}" >/dev/null
[[ "$(cat "${tmp_value}")" == "super-secret" ]] || { echo "[010-smoke] secret mismatch" >&2; exit 1; }

echo "[010-smoke] creating and using named store"
vaultline_run_store store init project-a "${VAULTLINE_TEST_PROJECT_STORE}" >/dev/null
vaultline_run_store store unseal project-a >/dev/null
printf "project-secret" | vaultline_run secret set project-a:smoke.token --stdin >/dev/null
vaultline_run secret get project-a:smoke.token --out "${tmp_value}" >/dev/null
[[ "$(cat "${tmp_value}")" == "project-secret" ]] || { echo "[010-smoke] project store secret mismatch" >&2; exit 1; }

echo "[010-smoke] ok"
