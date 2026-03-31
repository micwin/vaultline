#!/usr/bin/env bash
set -euo pipefail

TEST_ROOT="${SMOKEY_TEST_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
if [[ "$(basename "${TEST_ROOT}")" != "tests.d" ]]; then
  TEST_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
fi
PROJECT_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
STATE_DIR="${PROJECT_ROOT}/.testrun"
ENV_FILE="${STATE_DIR}/env"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[060-zip-roundtrip] missing env file" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "${ENV_FILE}"
ADDR="${VAULTLINE_TEST_ADDR:-127.0.0.1:19428}"
STORE_CONFIG_FILE="${VAULTLINE_TEST_STORE_CONFIG:-${STATE_DIR}/config/stores.json}"
STORE_CONFIG_HOME="$(dirname "${STORE_CONFIG_FILE}")"
STORE_DATA_HOME="${STATE_DIR}/data"
ZIP_FILE="${STATE_DIR}/roundtrip-default.zip"
SOURCE_STORE="roundtrip-src"
TARGET_STORE="roundtrip-dst"
SOURCE_STORE_DIR="${STATE_DIR}/stores/${SOURCE_STORE}"
TARGET_STORE_DIR="${STORE_DATA_HOME}/vaultline/stores/${TARGET_STORE}"

if [[ -n "${VAULTLINE_TEST_BIN:-}" ]]; then
  VAULTLINE_CLI=("${VAULTLINE_TEST_BIN}")
else
  VAULTLINE_CLI=(go run ./cmd/vaultline)
fi

run_cli() {
  XDG_CONFIG_HOME="${STORE_CONFIG_HOME}" XDG_DATA_HOME="${STORE_DATA_HOME}" "${VAULTLINE_CLI[@]}" --addr "${ADDR}" "$@"
}

init_output="$(run_cli store init "${SOURCE_STORE}" "${SOURCE_STORE_DIR}" 2>/dev/null)"
SOURCE_KEY="$(awk -F': ' '/^unseal key: / {print $2}' <<<"${init_output}")"
if [[ -z "${SOURCE_KEY}" ]]; then
  echo "[060-zip-roundtrip] failed to capture source unseal key" >&2
  exit 1
fi

target_init_output="$(run_cli store init "${TARGET_STORE}" "${TARGET_STORE_DIR}" 2>/dev/null)"
TARGET_OLD_KEY="$(awk -F': ' '/^unseal key: / {print $2}' <<<"${target_init_output}")"
if [[ -z "${TARGET_OLD_KEY}" ]]; then
  echo "[060-zip-roundtrip] failed to capture target unseal key" >&2
  exit 1
fi

keys=(
  "${SOURCE_STORE}:demo.alpha"
  "${SOURCE_STORE}:demo.beta"
  "${SOURCE_STORE}:demo.gamma"
  "${SOURCE_STORE}:group.one"
  "${SOURCE_STORE}:group.two"
  "${SOURCE_STORE}:group.three"
)

values=(
  "value-alpha"
  "value-beta"
  "value-gamma"
  "value-one"
  "value-two"
  "value-three"
)

for i in "${!keys[@]}"; do
  printf '%s' "${values[$i]}" | run_cli secret set "${keys[$i]}" --stdin >/dev/null
done

printf '%s' "old-target-value" | run_cli secret set "${TARGET_STORE}:demo.alpha" --stdin >/dev/null
printf '%s' "old-extra-value" | run_cli secret set "${TARGET_STORE}:extra.only-in-target" --stdin >/dev/null

run_cli backup zip "${SOURCE_STORE}" "${ZIP_FILE}" >/dev/null
if run_cli restore zip "${ZIP_FILE}" "${TARGET_STORE}" >/dev/null 2>&1; then
  echo "[060-zip-roundtrip] restore without --overwrite unexpectedly succeeded" >&2
  exit 1
fi
run_cli restore zip "${ZIP_FILE}" "${TARGET_STORE}" --overwrite >/dev/null
VAULTLINE_PASSPHRASE="${SOURCE_KEY}" run_cli store unseal "${TARGET_STORE}" >/dev/null

source_json="$(run_cli --output json secret list "${SOURCE_STORE}:")"
target_json="$(run_cli --output json secret list "${TARGET_STORE}:")"

SOURCE_KEYS_FILE="${STATE_DIR}/source-keys.txt"
TARGET_KEYS_FILE="${STATE_DIR}/target-keys.txt"
trap 'rm -f "${SOURCE_KEYS_FILE}" "${TARGET_KEYS_FILE}"' EXIT

python3 - <<'PY' "${SOURCE_STORE}" "${TARGET_STORE}" "${source_json}" "${target_json}" >"${SOURCE_KEYS_FILE}"
import json, sys
source_store, target_store, source_json, target_json = sys.argv[1:5]
source = json.loads(source_json)["keys"]
target = json.loads(target_json)["keys"]
source_names = sorted([f"{source_store}:{entry['name']}" for entry in source])
target_names = sorted([f"{target_store}:{entry['name']}" for entry in target])
for name in source_names:
    print(name)
print('---')
for name in target_names:
    print(name)
PY

sed -n '1,/^---$/p' "${SOURCE_KEYS_FILE}" | sed '$d' >"${TARGET_KEYS_FILE}.src"
sed -n '/^---$/,$p' "${SOURCE_KEYS_FILE}" | sed '1d' >"${TARGET_KEYS_FILE}.dst"

python3 - <<'PY' "${TARGET_KEYS_FILE}.src" "${TARGET_KEYS_FILE}.dst"
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
  src_value="$(run_cli --output raw secret get "${keys[$i]}")"
  dst_key="${keys[$i]/${SOURCE_STORE}:/${TARGET_STORE}:}"
  dst_value="$(run_cli --output raw secret get "${dst_key}")"
  if [[ "${src_value}" != "${dst_value}" ]]; then
    echo "[060-zip-roundtrip] value mismatch for ${keys[$i]}" >&2
    exit 1
  fi
done

if run_cli --output raw secret get "${TARGET_STORE}:extra.only-in-target" >/dev/null 2>&1; then
  echo "[060-zip-roundtrip] overwrite restore kept stale target-only key" >&2
  exit 1
fi

rm -f "${TARGET_KEYS_FILE}.src" "${TARGET_KEYS_FILE}.dst"
echo "[060-zip-roundtrip] ok"
