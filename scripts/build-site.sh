#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC_DIR="${ROOT_DIR}/site-src"
OUT_DIR="${ROOT_DIR}/site"

if [[ ! -d "${SRC_DIR}" ]]; then
  echo "build-site: site-src not found" >&2
  exit 1
fi

VERSION=$(grep 'const Version' "${ROOT_DIR}/pkg/version/version.go" | awk -F'"' '{print $2}')
if [[ -z "${VERSION}" ]]; then
  echo "build-site: unable to read version" >&2
  exit 1
fi

if ! command -v bundle >/dev/null 2>&1; then
  echo "build-site: bundler not found (install ruby + bundler)" >&2
  exit 1
fi

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

pushd "${SRC_DIR}" >/dev/null
BUNDLE_PATH="${ROOT_DIR}/dist/bundle" bundle install

TMP_CONFIG="${ROOT_DIR}/dist/site-config.yml"
cp _config.yml "${TMP_CONFIG}"
{
  echo "vaultline_version: \"${VERSION}\""
} >> "${TMP_CONFIG}"

BUNDLE_PATH="${ROOT_DIR}/dist/bundle" bundle exec jekyll build --config "${TMP_CONFIG}" --destination "${OUT_DIR}"
popd >/dev/null

echo "Built Jekyll site for vaultline ${VERSION} -> ${OUT_DIR}"
