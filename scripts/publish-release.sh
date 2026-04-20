#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

if [[ "$(git symbolic-ref --short HEAD)" != release* ]]; then
  echo "publish-release.sh: run from release branch" >&2
  exit 1
fi

if ! git diff --quiet --exit-code || ! git diff --quiet --exit-code --cached; then
  echo "publish-release.sh: working tree not clean" >&2
  exit 1
fi

git push origin "$(git symbolic-ref --short HEAD)"
echo "release branch pushed; GitHub Actions will publish release + pages"

