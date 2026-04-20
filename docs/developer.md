# Developer ramp-up

Developer onboarding for local work on vaultline.

## Table of contents

- [Prerequisites](#prerequisites)
- [Initial setup](#initial-setup)
- [Test workflow](#test-workflow)
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
