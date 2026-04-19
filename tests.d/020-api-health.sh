#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"
vaultline_require_setup

echo "[020-api-health] checking CLI health output"
cli_output="$(vaultline_run health)"
echo "[020-api-health] cli: ${cli_output}"

echo "[020-api-health] querying REST health endpoint"
http_output="$(curl -fsS "http://${VAULTLINE_TEST_ADDR}/api/v1/health")"
echo "[020-api-health] rest: ${http_output}"

grep -q '"sealed":false' <<<"${http_output}" || { echo "[020-api-health] daemon unexpectedly sealed" >&2; exit 1; }

echo "[020-api-health] ok"
