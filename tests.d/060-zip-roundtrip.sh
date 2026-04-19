#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"
vaultline_require_setup

zip_file="${SMOKEY_STATE_DIR}/roundtrip-default.zip"
source_store="roundtrip-src"
target_store="roundtrip-dst"
source_store_dir="${SMOKEY_STATE_DIR}/stores/${source_store}"
target_store_dir="${VAULTLINE_TEST_STORE_DATA}/vaultline/stores/${target_store}"

init_output="$(vaultline_run_store store init "${source_store}" "${source_store_dir}" 2>/dev/null)"
source_key="$(awk -F': ' '/^unseal key: / {print $2}' <<<"${init_output}")"
[[ -n "${source_key}" ]] || { echo "[060-zip-roundtrip] failed to capture source unseal key" >&2; exit 1; }

target_init_output="$(vaultline_run_store store init "${target_store}" "${target_store_dir}" 2>/dev/null)"
target_old_key="$(awk -F': ' '/^unseal key: / {print $2}' <<<"${target_init_output}")"
[[ -n "${target_old_key}" ]] || { echo "[060-zip-roundtrip] failed to capture target unseal key" >&2; exit 1; }

keys=(
  "${source_store}:demo.alpha"
  "${source_store}:demo.beta"
  "${source_store}:demo.gamma"
  "${source_store}:group.one"
  "${source_store}:group.two"
  "${source_store}:group.three"
)
values=(value-alpha value-beta value-gamma value-one value-two value-three)

for i in "${!keys[@]}"; do
  printf '%s' "${values[$i]}" | vaultline_run_store secret set "${keys[$i]}" --stdin >/dev/null
done

printf '%s' "old-target-value" | vaultline_run_store secret set "${target_store}:demo.alpha" --stdin >/dev/null
printf '%s' "old-extra-value" | vaultline_run_store secret set "${target_store}:extra.only-in-target" --stdin >/dev/null

vaultline_run_store backup zip "${source_store}" "${zip_file}" >/dev/null
vaultline_run_store restore zip "${zip_file}" "${target_store}" >/dev/null 2>&1 && { echo "[060-zip-roundtrip] restore without --overwrite unexpectedly succeeded" >&2; exit 1; }
vaultline_run_store restore zip "${zip_file}" "${target_store}" --overwrite >/dev/null
VAULTLINE_PASSPHRASE="${source_key}" vaultline_run_store store unseal "${target_store}" >/dev/null

source_json="$(vaultline_run_store --output json secret list "${source_store}:")"
target_json="$(vaultline_run_store --output json secret list "${target_store}:")"

source_keys_file="${SMOKEY_STATE_DIR}/source-keys.txt"
target_keys_file="${SMOKEY_STATE_DIR}/target-keys.txt"
trap 'rm -f "${source_keys_file}" "${target_keys_file}" "${target_keys_file}.src" "${target_keys_file}.dst"' EXIT

python3 - <<'PY' "${source_store}" "${target_store}" "${source_json}" "${target_json}" >"${source_keys_file}"
import json, sys
source_store, target_store, source_json, target_json = sys.argv[1:5]
source = json.loads(source_json)["keys"]
target = json.loads(target_json)["keys"]
for name in sorted([f"{source_store}:{entry['name']}" for entry in source]):
    print(name)
print('---')
for name in sorted([f"{target_store}:{entry['name']}" for entry in target]):
    print(name)
PY

sed -n '1,/^---$/p' "${source_keys_file}" | sed '$d' >"${target_keys_file}.src"
sed -n '/^---$/,$p' "${source_keys_file}" | sed '1d' >"${target_keys_file}.dst"

python3 - <<'PY' "${target_keys_file}.src" "${target_keys_file}.dst"
import sys
src = [line.strip() for line in open(sys.argv[1]) if line.strip()]
dst = [line.strip() for line in open(sys.argv[2]) if line.strip()]
normalized_dst = [line.replace('roundtrip-dst:', 'roundtrip-src:', 1) for line in dst]
if src != normalized_dst:
    print('source keys:', src)
    print('target keys:', normalized_dst)
    raise SystemExit(1)
PY

for i in "${!keys[@]}"; do
  src_value="$(vaultline_run_store --output raw secret get "${keys[$i]}")"
  dst_key="${keys[$i]/${source_store}:/${target_store}:}"
  dst_value="$(vaultline_run_store --output raw secret get "${dst_key}")"
  [[ "${src_value}" == "${dst_value}" ]] || { echo "[060-zip-roundtrip] value mismatch for ${keys[$i]}" >&2; exit 1; }
done

vaultline_run_store --output raw secret get "${target_store}:extra.only-in-target" >/dev/null 2>&1 && { echo "[060-zip-roundtrip] overwrite restore kept stale target-only key" >&2; exit 1; }

echo "[060-zip-roundtrip] ok"
