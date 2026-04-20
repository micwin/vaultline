#!/usr/bin/env bash
set -euo pipefail

if ! command -v apt-get >/dev/null 2>&1; then
  echo "This script targets Debian/Ubuntu derivatives." >&2
  exit 1
fi

PACKAGES=(
  build-essential
  curl
  git
  golang
  make
)

echo "[bootstrap] updating apt sources"
sudo apt-get update -y
echo "[bootstrap] installing packages: ${PACKAGES[*]}"
sudo apt-get install -y "${PACKAGES[@]}"

echo "[bootstrap] installing smokey (local path)"
(cd "$(dirname "${BASH_SOURCE[0]}")/.." && ../smokey/install.sh)

cat <<'INSTRUCTIONS'

Bootstrap complete.
- Run `go version` to confirm toolchain availability.
- From vaultline/, run `go test ./...` and `smokey --tests-dir tests.d` to validate the setup.

INSTRUCTIONS
