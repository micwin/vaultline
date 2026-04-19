#!/usr/bin/env bash

vaultline_require_setup() {
  if [[ -z "${VAULTLINE_TEST_ADDR:-}" ]]; then
    echo "[vaultline-tests] missing persisted setup env; 000-setup must run first" >&2
    exit 1
  fi
}

vaultline_run() {
  go run ./cmd/vaultline --addr "${VAULTLINE_TEST_ADDR}" "$@"
}

vaultline_run_store() {
  XDG_CONFIG_HOME="$(dirname "${VAULTLINE_TEST_STORE_CONFIG}")" \
  XDG_DATA_HOME="${VAULTLINE_TEST_STORE_DATA}" \
  vaultline_run "$@"
}
