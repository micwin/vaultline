#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"
vaultline_require_setup

invalid_zip="${SMOKEY_TEST_ROOT}/../testdata/backup-restore/not-a-zip.txt"
target_store="invalid-import-target"

vaultline_run_store restore zip "${invalid_zip}" "${target_store}" --overwrite >/dev/null 2>&1 && {
  echo "[061-import-zip-invalid] restore unexpectedly succeeded for invalid archive" >&2
  exit 1
}

vaultline_run_store import zip "${invalid_zip}" "${target_store}" --overwrite >/dev/null 2>&1 && {
  echo "[061-import-zip-invalid] import unexpectedly succeeded for invalid archive" >&2
  exit 1
}

echo "[061-import-zip-invalid] ok"
