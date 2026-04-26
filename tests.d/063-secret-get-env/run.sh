#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"
vaultline_require_setup

key="default:env.demo"
value="alpha 'beta' gamma"

printf '%s' "${value}" | vaultline_run_store secret set "${key}" --stdin >/dev/null

assignment="$(vaultline_run_store --output eval-set secret get "${key}" TOKEN)"
[[ "${assignment}" == "TOKEN='alpha '\''beta'\'' gamma'" ]] || {
  echo "[063-secret-get-env] unexpected assignment output: ${assignment}" >&2
  exit 1
}

unset TOKEN
eval "${assignment}"
[[ "${TOKEN}" == "${value}" ]] || {
  echo "[063-secret-get-env] eval did not set TOKEN correctly" >&2
  exit 1
}

export_assignment="$(vaultline_run_store --output eval-export secret get "${key}" TOKEN)"
[[ "${export_assignment}" == "TOKEN='alpha '\''beta'\'' gamma'; export TOKEN;" ]] || {
  echo "[063-secret-get-env] unexpected export assignment: ${export_assignment}" >&2
  exit 1
}

unset TOKEN
eval "${export_assignment}"
[[ "${TOKEN}" == "${value}" ]] || {
  echo "[063-secret-get-env] export eval did not set TOKEN correctly" >&2
  exit 1
}

env | grep -q "^TOKEN=alpha 'beta' gamma$" || {
  echo "[063-secret-get-env] TOKEN was not exported" >&2
  exit 1
}

echo "[063-secret-get-env] ok"
