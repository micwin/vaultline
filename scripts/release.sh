#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

if [[ "$(git symbolic-ref --short HEAD)" != "develop" ]]; then
  echo "release.sh: please run from develop" >&2
  exit 1
fi

if ! git diff --quiet --exit-code || ! git diff --quiet --exit-code --cached; then
  echo "release.sh: working tree not clean" >&2
  exit 1
fi

git fetch origin

if git show-ref --verify --quiet refs/heads/release; then
  git checkout release
  git pull --ff-only origin release
elif git show-ref --verify --quiet refs/remotes/origin/release; then
  git checkout -b release origin/release
  git pull --ff-only origin release
else
  git checkout -b release develop
  git push -u origin release
fi

git merge --ff-only develop
git push origin release
git checkout develop

echo "release branch updated; watch GitHub Actions"

