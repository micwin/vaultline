# Developer ramp-up

Developer onboarding for local work on vaultline.

## Table of contents

- [Prerequisites](#prerequisites)
- [Initial setup](#initial-setup)
- [Test workflow](#test-workflow)
- [Build workflow](#build-workflow)
- [Site/docs workflow](#sitedocs-workflow)
- [Release handoff expectations](#release-handoff-expectations)

## Prerequisites

- Go 1.22+
- Python + MkDocs Material (for docs build)
- Ruby + Bundler (for Jekyll pages build)

## Initial setup

```bash
./scripts/bootstrap-debian.sh
```

This installs core tooling and [Smokey](https://github.com/micwin/smokey).

## Test workflow

```bash
go test ./...
smokey --tests-dir tests.d
```

Use focused package tests first, then full suite before release prep.

## Build workflow

Default build (compile + deb package):

```bash
./scripts/build.sh
```

Targeted builds:

```bash
./scripts/build.sh --compile
./scripts/build.sh --deb
./scripts/build.sh --site
./scripts/build.sh --clean --deb --site
```

Version bump during build:

```bash
./scripts/build.sh --deb --version X.Y.Z
```

Release guardrail:

- Do not bump versions directly on `release` branch.
- Perform version bump and release prep on `develop` via `./scripts/prepare-release.sh <X.Y.Z>`.

## Site/docs workflow

```bash
./scripts/build-site.sh
./scripts/serve-site.sh
```

`build-site.sh` produces `site/` with Jekyll + MkDocs output.

## Release handoff expectations

- Work on `develop`.
- Keep release-note snippets under `site-src/release-notes/unreleased/`.
- Hand off changes via PR/commit before running release scripts.
- Use [Runbook (Project Workflow)](runbook.md) for exact release sequence.
