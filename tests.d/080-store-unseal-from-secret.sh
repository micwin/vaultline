#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"
vaultline_require_setup

source_store="source-store"
source_dir="${SMOKEY_STATE_DIR}/stores/${source_store}"
target_store="target-store"
target_dir="${SMOKEY_STATE_DIR}/stores/${target_store}"
secret_ref="${source_store}:target.unseal"

target_init="$(vaultline_run_store store init "${target_store}" "${target_dir}" 2>/dev/null)"
target_key="$(awk -F': ' '/^unseal key: / {print $2}' <<<"${target_init}")"
[[ -n "${target_key}" ]] || { echo "[080-unseal-from-secret] missing target unseal key" >&2; exit 1; }

vaultline_run_store store init "${source_store}" "${source_dir}" >/dev/null 2>&1 || true
printf '%s\n' "${target_key}" | vaultline_run_store secret set "${secret_ref}" --stdin >/dev/null

vaultline_run_store store seal "${target_store}" >/dev/null
if vaultline_run_store store unseal "${target_store}" --value "wrong-${target_key}" >/dev/null 2>&1; then
  echo "[080-unseal-from-secret] wrong passphrase unexpectedly unsealed target store" >&2
  exit 1
fi
vaultline_run_store store unseal "${target_store}" --from-secret "${secret_ref}" >/dev/null

value_file="$(mktemp "${SMOKEY_STATE_DIR}/unseal-from-secret.XXXX")"
trap 'rm -f "${value_file}"' EXIT
printf 'ok' | vaultline_run_store secret set "${target_store}:unseal.check" --stdin >/dev/null
vaultline_run_store secret get "${target_store}:unseal.check" --out "${value_file}" >/dev/null
[[ "$(cat "${value_file}")" == "ok" ]] || { echo "[080-unseal-from-secret] target store still sealed" >&2; exit 1; }

echo "[080-unseal-from-secret] ok"
