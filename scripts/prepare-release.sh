#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

VERSION="${1:-}"

if [[ -n "${VERSION}" ]]; then
  ./scripts/build.sh --deb --version "${VERSION}"
else
  ./scripts/build.sh --deb
fi

VERSION="$(grep 'const Version' pkg/version/version.go | awk -F'"' '{print $2}')"
if [[ -z "${VERSION}" ]]; then
  echo "prepare-release: unable to read version" >&2
  exit 1
fi

NOTES_ROOT="${ROOT_DIR}/site-src/release-notes"
UNRELEASED_DIR="${NOTES_ROOT}/unreleased"
RELEASE_NOTE_FILE="${NOTES_ROOT}/v${VERSION}.md"
RELEASES_JSON="${ROOT_DIR}/site-src/_data/releases.json"
CURRENT_JSON="${ROOT_DIR}/site-src/_data/current.json"

mkdir -p "${UNRELEASED_DIR}" "${NOTES_ROOT}" "${ROOT_DIR}/site-src/_data"

python3 - <<'PY' "${UNRELEASED_DIR}" "${RELEASE_NOTE_FILE}" "${VERSION}" "${RELEASES_JSON}" "${CURRENT_JSON}"
import datetime
import json
import pathlib
import re
import sys

unreleased_dir = pathlib.Path(sys.argv[1])
release_note_file = pathlib.Path(sys.argv[2])
version = sys.argv[3]
releases_json = pathlib.Path(sys.argv[4])
current_json = pathlib.Path(sys.argv[5])

snippets = sorted([p for p in unreleased_dir.glob("*.md") if p.is_file()])

changes = []
for snippet in snippets:
    text = snippet.read_text().strip()
    if not text:
        continue
    for line in text.splitlines():
        line = line.strip()
        if not line:
            continue
        if line.startswith("- "):
            changes.append(line[2:].strip())
        else:
            changes.append(line)

if not changes:
    changes = ["Release preparation without explicit unreleased notes."]

if not release_note_file.exists():
    body = "\n".join([f"- {item}" for item in changes])
    release_note_file.write_text(
        "---\n"
        "layout: default\n"
        f"title: Release Notes v{version}\n"
        "---\n\n"
        f"# Release Notes v{version}\n\n"
        "## Changes\n\n"
        f"{body}\n"
    )

for snippet in snippets:
    snippet.unlink(missing_ok=True)

if releases_json.exists():
    try:
        releases = json.loads(releases_json.read_text())
        if not isinstance(releases, list):
            releases = []
    except Exception:
        releases = []
else:
    releases = []

version_tag = f"v{version}"
entry = {
    "version": version_tag,
    "published": datetime.date.today().isoformat(),
    "deb_url": f"https://github.com/micwin/vaultline/releases/download/{version_tag}/vaultline_{version}_amd64.deb",
    "binary_url": f"https://github.com/micwin/vaultline/releases/download/{version_tag}/vaultline",
    "release_url": f"https://github.com/micwin/vaultline/releases/tag/{version_tag}",
    "notes_url": f"/release-notes/v{version}/"
}

releases = [r for r in releases if r.get("version") != version_tag]
releases.insert(0, entry)
releases_json.write_text(json.dumps(releases, indent=2) + "\n")

current_payload = {
    "version": version,
    "changes": changes
}
current_json.write_text(json.dumps(current_payload, indent=2) + "\n")
PY

./scripts/build.sh --site

echo "prepare-release complete"
echo "- review changes"
echo "- commit them"
echo "- run ./scripts/release.sh"
