#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"
vaultline_require_setup

zip_file="${SMOKEY_STATE_DIR}/roundtrip-export-import.zip"
source_store="roundtrip-export-src"
target_store="roundtrip-import-dst"
source_store_dir="${SMOKEY_STATE_DIR}/stores/${source_store}"
target_store_dir="${VAULTLINE_TEST_STORE_DATA}/vaultline/stores/${target_store}"

init_output="$(vaultline_run_store store init "${source_store}" "${source_store_dir}" 2>/dev/null)"
source_key="$(awk -F': ' '/^unseal key: / {print $2}' <<<"${init_output}")"
[[ -n "${source_key}" ]] || { echo "[062-export-import-roundtrip] failed to capture source unseal key" >&2; exit 1; }

vaultline_run_store store init "${target_store}" "${target_store_dir}" >/dev/null

printf '%s' "value-alpha" | vaultline_run_store secret set "${source_store}:demo.alpha" --stdin >/dev/null
printf '%s' "value-beta" | vaultline_run_store secret set "${source_store}:demo.beta" --stdin >/dev/null
printf '%s' "value-gamma" | vaultline_run_store secret set "${source_store}:group.gamma" --stdin >/dev/null

printf '%s' "old-target-value" | vaultline_run_store secret set "${target_store}:demo.alpha" --stdin >/dev/null
printf '%s' "old-extra-value" | vaultline_run_store secret set "${target_store}:extra.only-in-target" --stdin >/dev/null

vaultline_run_store export zip "${source_store}" "${zip_file}" >/dev/null

vaultline_run_store import zip "${zip_file}" "${target_store}" >/dev/null 2>&1 && {
  echo "[062-export-import-roundtrip] import without --overwrite unexpectedly succeeded" >&2
  exit 1
}

vaultline_run_store import zip "${zip_file}" "${target_store}" --overwrite >/dev/null
VAULTLINE_PASSPHRASE="${source_key}" vaultline_run_store store unseal "${target_store}" >/dev/null

src_alpha="$(vaultline_run_store --output raw secret get "${source_store}:demo.alpha")"
dst_alpha="$(vaultline_run_store --output raw secret get "${target_store}:demo.alpha")"
[[ "${src_alpha}" == "${dst_alpha}" ]] || { echo "[062-export-import-roundtrip] value mismatch for demo.alpha" >&2; exit 1; }

src_beta="$(vaultline_run_store --output raw secret get "${source_store}:demo.beta")"
dst_beta="$(vaultline_run_store --output raw secret get "${target_store}:demo.beta")"
[[ "${src_beta}" == "${dst_beta}" ]] || { echo "[062-export-import-roundtrip] value mismatch for demo.beta" >&2; exit 1; }

src_gamma="$(vaultline_run_store --output raw secret get "${source_store}:group.gamma")"
dst_gamma="$(vaultline_run_store --output raw secret get "${target_store}:group.gamma")"
[[ "${src_gamma}" == "${dst_gamma}" ]] || { echo "[062-export-import-roundtrip] value mismatch for group.gamma" >&2; exit 1; }

vaultline_run_store --output raw secret get "${target_store}:extra.only-in-target" >/dev/null 2>&1 && {
  echo "[062-export-import-roundtrip] overwrite import kept stale target-only key" >&2
  exit 1
}

echo "[062-export-import-roundtrip] ok"
