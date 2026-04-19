#!/usr/bin/env bash
set -euo pipefail

source "${SMOKEY_TEST_ROOT}/vaultline-testlib.sh"
vaultline_require_setup

key_name="list.fire-test"
value="embers-${RANDOM}"
project_key_name="list.project-test"
project_value="project-${RANDOM}"

echo "${value}" | vaultline_run secret set "default:${key_name}" --stdin >/dev/null
vaultline_run_store store init project-a "${VAULTLINE_TEST_PROJECT_STORE}" >/dev/null 2>&1 || true
vaultline_run_store store unseal project-a >/dev/null 2>&1 || true
echo "${project_value}" | vaultline_run secret set "project-a:${project_key_name}" --stdin >/dev/null

output_local="$(vaultline_run secret list default:)"
grep -q "default:${key_name}" <<<"${output_local}" || { echo "[050-secrets-list] default:${key_name} missing in output" >&2; exit 1; }
grep -q "project-a:${project_key_name}" <<<"${output_local}" && { echo "[050-secrets-list] project key leaked into local list" >&2; exit 1; }

output_project="$(vaultline_run secret list project-a:)"
grep -q "project-a:${project_key_name}" <<<"${output_project}" || { echo "[050-secrets-list] project-a:${project_key_name} missing in output" >&2; exit 1; }

echo "[050-secrets-list] ok"
