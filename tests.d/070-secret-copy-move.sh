#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"
vaultline_require_setup

store_a="copy-src"
store_b="copy-dst"
store_a_dir="${SMOKEY_STATE_DIR}/stores/${store_a}"
store_b_dir="${SMOKEY_STATE_DIR}/stores/${store_b}"

testdata_dir="${VAULTLINE_TEST_FIXTURES}/secret-copy-move"
src_value_file="${testdata_dir}/source-value.txt"
dst_value_file="${testdata_dir}/destination-value.txt"

work_dir="${SMOKEY_STATE_DIR}/copy-move"
mkdir -p "${work_dir}"
copied_out="${work_dir}/copied.out"
moved_out="${work_dir}/moved.out"
src_out="${work_dir}/source.out"
err_out="${work_dir}/copy-collision.err"

vaultline_run store init "${store_a}" "${store_a_dir}" >/dev/null
vaultline_run store init "${store_b}" "${store_b_dir}" >/dev/null
vaultline_run secret set "${store_a}:alpha" --file "${src_value_file}" >/dev/null
vaultline_run secret set "${store_b}:alpha" --file "${dst_value_file}" >/dev/null

echo "[070-secret-copy-move] expecting copy collision to fail without --force"
vaultline_run secret copy "${store_a}:alpha" "${store_b}:alpha" >"${work_dir}/copy-collision.out" 2>"${err_out}" && {
  echo "[070-secret-copy-move] copy unexpectedly succeeded despite destination collision" >&2
  exit 1
}
grep -qi "exists\|collision\|already" "${err_out}" || { echo "[070-secret-copy-move] expected collision/error hint in stderr" >&2; cat "${err_out}" >&2; exit 1; }

vaultline_run secret get "${store_b}:alpha" --out "${copied_out}" >/dev/null
cmp -s "${copied_out}" "${dst_value_file}" || { echo "[070-secret-copy-move] destination changed although copy should have failed" >&2; exit 1; }

echo "[070-secret-copy-move] forcing copy over existing destination"
vaultline_run secret copy "${store_a}:alpha" "${store_b}:alpha" --force >/dev/null
vaultline_run secret get "${store_b}:alpha" --out "${copied_out}" >/dev/null
cmp -s "${copied_out}" "${src_value_file}" || { echo "[070-secret-copy-move] forced copy did not write source value" >&2; exit 1; }
vaultline_run secret get "${store_a}:alpha" --out "${src_out}" >/dev/null
cmp -s "${src_out}" "${src_value_file}" || { echo "[070-secret-copy-move] copy should not remove source secret" >&2; exit 1; }

echo "[070-secret-copy-move] moving secret to new key"
vaultline_run secret move "${store_a}:alpha" "${store_b}:beta" >/dev/null
vaultline_run secret get "${store_b}:beta" --out "${moved_out}" >/dev/null
cmp -s "${moved_out}" "${src_value_file}" || { echo "[070-secret-copy-move] move destination mismatch" >&2; exit 1; }
vaultline_run secret get "${store_a}:alpha" --out "${src_out}" >/dev/null 2>&1 && { echo "[070-secret-copy-move] source still exists after move" >&2; exit 1; }

echo "[070-secret-copy-move] expecting move collision to fail without --force"
vaultline_run secret set "${store_a}:gamma" --file "${src_value_file}" >/dev/null
vaultline_run secret set "${store_b}:gamma" --file "${dst_value_file}" >/dev/null
vaultline_run secret move "${store_a}:gamma" "${store_b}:gamma" >"${work_dir}/move-collision.out" 2>"${err_out}" && {
  echo "[070-secret-copy-move] move unexpectedly succeeded despite destination collision" >&2
  exit 1
}
grep -qi "exists\|collision\|already" "${err_out}" || { echo "[070-secret-copy-move] expected move collision hint in stderr" >&2; cat "${err_out}" >&2; exit 1; }
vaultline_run secret get "${store_b}:gamma" --out "${copied_out}" >/dev/null
cmp -s "${copied_out}" "${dst_value_file}" || { echo "[070-secret-copy-move] destination changed although move should have failed" >&2; exit 1; }
vaultline_run secret get "${store_a}:gamma" --out "${src_out}" >/dev/null
cmp -s "${src_out}" "${src_value_file}" || { echo "[070-secret-copy-move] source changed although move should have failed" >&2; exit 1; }

echo "[070-secret-copy-move] forcing move over existing destination"
vaultline_run secret move "${store_a}:gamma" "${store_b}:gamma" --force >/dev/null
vaultline_run secret get "${store_b}:gamma" --out "${moved_out}" >/dev/null
cmp -s "${moved_out}" "${src_value_file}" || { echo "[070-secret-copy-move] forced move did not overwrite destination" >&2; exit 1; }
vaultline_run secret get "${store_a}:gamma" --out "${src_out}" >/dev/null 2>&1 && { echo "[070-secret-copy-move] source still exists after forced move" >&2; exit 1; }

echo "[070-secret-copy-move] ok"
