#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SITE_DIR="${ROOT_DIR}/site"
SERVE_ROOT="${ROOT_DIR}/dist/site-serve"
USER_SET_PORT="${PORT+x}"
PORT="${PORT:-4000}"

port_in_use() {
  local check_port="$1"
  ss -ltn "( sport = :${check_port} )" | awk 'NR>1 { found=1 } END { exit(found ? 0 : 1) }'
}

if ! command -v python3 >/dev/null 2>&1; then
  echo "ghpages-serve: python3 not found" >&2
  exit 1
fi

if [[ ! -f "${SITE_DIR}/index.html" ]]; then
  echo "ghpages-serve: built site not found, running build-site first"
  "${ROOT_DIR}/scripts/build-site.sh"
fi

rm -rf "${SERVE_ROOT}"
mkdir -p "${SERVE_ROOT}/vaultline"
cp -a "${SITE_DIR}/." "${SERVE_ROOT}/vaultline/"

if port_in_use "${PORT}"; then
  if [[ -n "${USER_SET_PORT}" ]]; then
    echo "ghpages-serve: requested port ${PORT} is already in use" >&2
    echo "Try another port, for example: PORT=4001 ./scripts/serve-site.sh" >&2
    exit 1
  fi

  for candidate in 4001 4002 4003 4004 4005 4010; do
    if ! port_in_use "${candidate}"; then
      PORT="${candidate}"
      break
    fi
  done

  if port_in_use "${PORT}"; then
    echo "ghpages-serve: no free fallback port found (tried 4000-4005,4010)" >&2
    echo "Set an explicit one: PORT=4011 ./scripts/serve-site.sh" >&2
    exit 1
  fi
fi

echo "Serving static site at http://127.0.0.1:${PORT}/vaultline/"
python3 -m http.server "${PORT}" --bind 127.0.0.1 --directory "${SERVE_ROOT}"
