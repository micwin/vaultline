# Releasing vaultline

This repo follows a `develop -> release` publishing flow.

## 1) Prepare artifacts
- Run `./scripts/prepare-release.sh` (optionally with explicit version: `./scripts/prepare-release.sh 0.3.32`).
- This runs the orchestrator `./scripts/build.sh --deb --site` (and `--version` when provided).
- Review and commit resulting changes on `develop`.

Jekyll notes:
- Pages source is `site-src/`.
- Local preview: `./scripts/ghpages-serve.sh`.
- Static output for deploy: `site/`.

## 2) Publish release branch
- Run `./scripts/release.sh` from a clean `develop` branch.
- The script fast-forwards `release` to `develop` and pushes it.

## 3) GitHub Actions does the rest
Workflow `.github/workflows/release.yml` (triggered by `release` push):
- builds artifacts via `./scripts/build.sh --deb --site --version <version>`
- creates/pushes tag `v<version>`
- publishes GitHub Release with `.deb` and raw binary
- deploys `site/` to GitHub Pages

## 4) Optional manual push helper
- If you are already on `release` and only need to push it: `./scripts/publish-release.sh`.
