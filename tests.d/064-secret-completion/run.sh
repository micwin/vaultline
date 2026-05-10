#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"
vaultline_require_setup

printf '%s' "leaf-value" | vaultline_run_store secret set "default:complete.branch.leaf" --stdin >/dev/null
printf '%s' "sibling-value" | vaultline_run_store secret set "default:complete.branch.sibling" --stdin >/dev/null
printf '%s' "direct-value" | vaultline_run_store secret set "default:complete.direct" --stdin >/dev/null
printf '%s' "sealed-value" | vaultline_run_store secret set "project-a:complete.sealed.leaf" --stdin >/dev/null

branch_output="$(vaultline_run_store __complete "default:complete." secret get)"
grep -qx "default:complete.branch." <<<"${branch_output}" || {
  echo "[064-secret-completion] missing branch prefix completion" >&2
  echo "${branch_output}" >&2
  exit 1
}
grep -qx "default:complete.direct" <<<"${branch_output}" || {
  echo "[064-secret-completion] missing direct leaf completion" >&2
  echo "${branch_output}" >&2
  exit 1
}

leaf_output="$(vaultline_run_store __complete "default:complete.branch." secret get)"
grep -qx "default:complete.branch.leaf" <<<"${leaf_output}" || {
  echo "[064-secret-completion] missing final leaf completion" >&2
  echo "${leaf_output}" >&2
  exit 1
}
grep -qx "default:complete.branch.sibling" <<<"${leaf_output}" || {
  echo "[064-secret-completion] missing final sibling completion" >&2
  echo "${leaf_output}" >&2
  exit 1
}

empty_current_output="$(vaultline_run_store __complete "" secret get "default:complete.branch.")"
grep -qx "default:complete.branch.leaf" <<<"${empty_current_output}" || {
  echo "[064-secret-completion] missing final leaf completion when shell passes empty current" >&2
  echo "${empty_current_output}" >&2
  exit 1
}

vaultline_run_store store seal project-a >/dev/null
sealed_output="$(vaultline_run_store __complete "project-a:complete.sealed." secret get)"
grep -qx "project-a:complete.sealed.leaf" <<<"${sealed_output}" || {
  echo "[064-secret-completion] missing file-backed completion for sealed store" >&2
  echo "${sealed_output}" >&2
  exit 1
}

echo "[064-secret-completion] ok"
