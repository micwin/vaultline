#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

VERSION="${1:-}"
if [[ -n "${VERSION}" ]]; then
  ./scripts/build.sh --deb --site --version "${VERSION}"
else
  ./scripts/build.sh --deb --site
fi

echo "prepare-release complete"
echo "- review changes"
echo "- commit them"
echo "- run ./scripts/release.sh"
