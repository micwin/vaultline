#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
PKG_DIR="${DIST_DIR}/debian"
ARCH="amd64"

DO_COMPILE=1
DO_SITE=0
DO_DEB=1
DO_INSTALL=0
DO_CLEAN=0
NEW_VERSION=""

usage() {
  cat <<'EOF'
Usage: ./scripts/build.sh [options]

Without flags, this runs compile + deb packaging.

Options:
  --compile            Build CLI binary into dist/
  --site               Build Pages content into site/
  --deb                Build Debian package into dist/
  --version X.Y.Z      Set pkg/version/version.go before building
  --install            Install generated .deb via sudo dpkg -i (implies --deb)
  --clean              Remove previous dist/ before build
  -h, --help           Show this help
EOF
}

if [[ $# -gt 0 ]]; then
  DO_COMPILE=0
  DO_SITE=0
  DO_DEB=0
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --compile)
      DO_COMPILE=1
      ;;
    --site)
      DO_SITE=1
      ;;
    --deb)
      DO_DEB=1
      ;;
    --install)
      DO_INSTALL=1
      DO_DEB=1
      ;;
    --clean)
      DO_CLEAN=1
      ;;
    --version)
      shift
      [[ $# -gt 0 ]] || { echo "--version requires value" >&2; exit 1; }
      NEW_VERSION="$1"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
  shift
done

if [[ "${DO_CLEAN}" -eq 1 ]]; then
  rm -rf "${DIST_DIR}"
fi
mkdir -p "${DIST_DIR}"

CURRENT_VERSION=$(grep 'const Version' "${ROOT_DIR}/pkg/version/version.go" | awk -F'"' '{print $2}')
if [[ -z "${CURRENT_VERSION}" ]]; then
  echo "build.sh: unable to read version" >&2
  exit 1
fi

if [[ -n "${NEW_VERSION}" ]]; then
  if [[ ! "${NEW_VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "build.sh: invalid version ${NEW_VERSION}" >&2
    exit 1
  fi
  if [[ "${NEW_VERSION}" != "${CURRENT_VERSION}" ]]; then
    sed -i "s/const Version = \"${CURRENT_VERSION}\"/const Version = \"${NEW_VERSION}\"/" "${ROOT_DIR}/pkg/version/version.go"
    CURRENT_VERSION="${NEW_VERSION}"
  fi
fi

cd "${ROOT_DIR}"

if [[ "${DO_COMPILE}" -eq 1 || "${DO_DEB}" -eq 1 ]]; then
  go test ./...
  CGO_ENABLED=0 go build -o "${DIST_DIR}/vaultline" ./cmd/vaultline
fi

if [[ "${DO_SITE}" -eq 1 ]]; then
  if ! command -v bundle >/dev/null 2>&1; then
    echo "build.sh: bundler missing; install ruby/bundler for --site" >&2
    exit 1
  fi
  ./scripts/build-site.sh
fi

if [[ "${DO_DEB}" -eq 1 ]]; then
  rm -rf "${PKG_DIR}"
  mkdir -p "${PKG_DIR}/usr/local/bin" "${PKG_DIR}/usr/lib/systemd/system" "${PKG_DIR}/usr/share/doc/vaultline" "${PKG_DIR}/DEBIAN"
  cp "${DIST_DIR}/vaultline" "${PKG_DIR}/usr/local/bin/vaultline"
  cp "${ROOT_DIR}/systemd/vaultline@.service" "${PKG_DIR}/usr/lib/systemd/system/vaultline@.service"
  cp "${ROOT_DIR}/README.md" "${PKG_DIR}/usr/share/doc/vaultline/README"
  cp "${ROOT_DIR}/debian/postinst" "${PKG_DIR}/DEBIAN/postinst"
  cp "${ROOT_DIR}/debian/prerm" "${PKG_DIR}/DEBIAN/prerm"
  cp "${ROOT_DIR}/debian/postrm" "${PKG_DIR}/DEBIAN/postrm"

  cat > "${PKG_DIR}/DEBIAN/control" <<CTRL
Package: vaultline
Version: ${CURRENT_VERSION}
Section: utils
Priority: optional
Architecture: ${ARCH}
Maintainer: vaultline
Description: Vaultline secret store unified binary
CTRL

  cd "${PKG_DIR}"
  find usr -type d -exec chmod 755 {} +
  chmod 755 usr/local/bin/vaultline
  chmod 644 usr/share/doc/vaultline/README

  cd "${DIST_DIR}"
  DEB_PATH="vaultline_${CURRENT_VERSION}_${ARCH}.deb"
  dpkg-deb --build debian "${DEB_PATH}"

  if [[ "${DO_INSTALL}" -eq 1 ]]; then
    sudo dpkg -i "${DEB_PATH}"
    hash -r 2>/dev/null || true
  fi
fi

echo "build.sh done (version ${CURRENT_VERSION})"
