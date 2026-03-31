#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
PKG_DIR="${DIST_DIR}/debian"
INSTALL_DEB=false
NEW_VERSION=""

usage() {
  cat <<'EOF'
Usage: ./package-deb.sh [VERSION] [--install]

Options:
  VERSION    Optional semantic version to write into `pkg/version/version.go`
  --install  Install the freshly built Debian package via `sudo dpkg -i`
  -h, --help Show this help text
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install)
      INSTALL_DEB=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      if [[ -n "${NEW_VERSION}" ]]; then
        echo "Unexpected argument: $1" >&2
        usage
        exit 1
      fi
      NEW_VERSION="$1"
      shift
      ;;
  esac
done

CURRENT_VERSION=$(grep 'const Version' "${ROOT_DIR}/pkg/version/version.go" | awk -F'"' '{print $2}')
if [[ -z "${CURRENT_VERSION}" ]]; then
  echo "Unable to read version from pkg/version/version.go" >&2
  exit 1
fi

if [[ -z "${NEW_VERSION}" ]]; then
  IFS='.' read -r major minor patch <<<"${CURRENT_VERSION}"
  patch=$((patch + 1))
  NEW_VERSION="${major}.${minor}.${patch}"
fi

if [[ ! "${NEW_VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Invalid version: ${NEW_VERSION}" >&2
  exit 1
fi

if [[ "${NEW_VERSION}" != "${CURRENT_VERSION}" ]]; then
  sed -i "s/const Version = \"${CURRENT_VERSION}\"/const Version = \"${NEW_VERSION}\"/" "${ROOT_DIR}/pkg/version/version.go"
fi

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

cd "${ROOT_DIR}"
go test ./...
CGO_ENABLED=0 go build -o "${DIST_DIR}/vaultline" ./cmd/vaultline

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
Version: ${NEW_VERSION}
Section: utils
Priority: optional
Architecture: amd64
Maintainer: vaultline
Description: Vaultline secret store unified binary
CTRL

cd "${PKG_DIR}"
find usr -type d -exec chmod 755 {} +
chmod 755 usr/local/bin/vaultline
chmod 644 usr/share/doc/vaultline/README

cd "${DIST_DIR}"
DEB_PATH="vaultline_${NEW_VERSION}_amd64.deb"
dpkg-deb --build debian "${DEB_PATH}"

if [[ "${INSTALL_DEB}" == "true" ]]; then
  sudo dpkg -i "${DEB_PATH}"
fi

echo "Built vaultline ${NEW_VERSION}"
