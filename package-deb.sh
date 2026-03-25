#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
PKG_DIR="${DIST_DIR}/debian"
mkdir -p "${DIST_DIR}"

VERSION=$(grep 'const Version' "${ROOT_DIR}/pkg/version/version.go" | awk -F'"' '{print $2}')
IFS='.' read -r major minor patch <<<"${VERSION}"
patch=$((patch+1))
NEW_VERSION="${major}.${minor}.${patch}"
sed -i "s/const Version = \"${VERSION}\"/const Version = \"${NEW_VERSION}\"/" "${ROOT_DIR}/pkg/version/version.go"

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
find usr -type f -exec chmod 755 {} +
cd "${DIST_DIR}"
dpkg-deb --build debian "vaultline-${NEW_VERSION}.deb"

echo "Built vaultline ${NEW_VERSION}"
