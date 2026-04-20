#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC_DIR="${ROOT_DIR}/site-src"

if ! command -v bundle >/dev/null 2>&1; then
  echo "ghpages-serve: bundler not found" >&2
  exit 1
fi

pushd "${SRC_DIR}" >/dev/null
BUNDLE_PATH="${ROOT_DIR}/dist/bundle" bundle install
BUNDLE_PATH="${ROOT_DIR}/dist/bundle" bundle exec jekyll serve --host 127.0.0.1 --port 4000
popd >/dev/null

