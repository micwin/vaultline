#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"
vaultline_require_setup

value_file="${SMOKEY_STATE_DIR}/kv-check.txt"
fixture="${SMOKEY_TEST_ROOT}/../testdata/secret-value.txt"
trap 'rm -f "${value_file}"' EXIT

[[ -f "${fixture}" ]] || { echo "[040-key-validation] missing fixture ${fixture}" >&2; exit 1; }

valid_name="mötor-1.gamma@local"
invalid_name="Uppercase"
invalid_slash_name="slash/name"

cat "${fixture}" | vaultline_run secret set --name "${valid_name}" --stdin >/dev/null
vaultline_run secret get --name "${valid_name}" --out "${value_file}" >/dev/null
cmp -s "${value_file}" "${fixture}" || { echo "[040-key-validation] retrieved secret mismatch" >&2; exit 1; }

vaultline_run secret set --name "${invalid_name}" --value bogus >/dev/null 2>&1 && { echo "[040-key-validation] invalid name unexpectedly succeeded" >&2; exit 1; }
vaultline_run secret set --name "${invalid_slash_name}" --value bogus >/dev/null 2>&1 && { echo "[040-key-validation] slash name unexpectedly succeeded" >&2; exit 1; }

echo "[040-key-validation] ok"
