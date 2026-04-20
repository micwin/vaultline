# Runbook (Project Workflow)

This runbook describes repository workflow and release operations.

## Table of contents

- [Development cycle](#development-cycle)
- [Versioning](#versioning)
- [Release flow](#release-flow)
- [Site/docs workflow](#sitedocs-workflow)
- [Helper scripts](#helper-scripts)
- [Operational hardening checklist](#operational-hardening-checklist)
- [Developer references](#developer-references)

## Development cycle

1. Work on `develop`.
2. Keep commits small and focused.
3. Run tests/build locally before release prep.

## Versioning

- Version is stored in `pkg/version/version.go`.
- Release prep updates this version and produces artifacts.
- Version bump is automatic as part of `./scripts/prepare-release.sh <X.Y.Z>`.
- Do release prep on `develop`, not directly on `release`, to avoid branch/history confusion and accidental out-of-sequence version updates.

## Release flow

1. Prepare release content and artifacts:

   ```bash
   ./scripts/prepare-release.sh <X.Y.Z>
   ```

2. Review and commit generated changes on `develop`.
3. Push `develop`.
4. Fast-forward/push `release`:

   ```bash
   ./scripts/release.sh
   ```

5. GitHub Actions builds `.deb` + binary, tags `vX.Y.Z`, publishes release, and deploys Pages.

Practical guardrail:

- Avoid manual version edits on `release` branch; always flow through `develop -> release`.

## Site/docs workflow

- Main site content: `site-src/` (Jekyll)
- Docs content: `docs/` (MkDocs Material)

Local build:

```bash
./scripts/build-site.sh
```

Local serve:

```bash
./scripts/serve-site.sh
```

## Helper scripts

- `scripts/build.sh` — central orchestrator for compile/package/site tasks.
- `scripts/build-site.sh` — builds static site output (`site/`) from Jekyll (`site-src/`) and MkDocs (`docs/`).
- `scripts/serve-site.sh` — convenience wrapper to build then serve locally.
- `scripts/ghpages-serve.sh` — static HTTP server for already-built output, including `/vaultline/docs/`.
- `scripts/prepare-release.sh` — bumps to target version, builds artifacts, merges unreleased notes into versioned notes, updates downloads metadata.
- `scripts/release.sh` — fast-forwards `release` from `develop` and pushes it to trigger CI release/publish.
- `scripts/publish-release.sh` — push helper when already on `release`.

## Operational hardening checklist

- Keep daemon listeners on loopback by default; only add remote binds deliberately.
- Use allowlists for every non-loopback listener (`daemon allow ...`), and review them regularly.
- Keep stores sealed by default and unseal only what is needed for the current task.
- Prefer cross-store unseal material (`store unseal --from-secret`) over manual passphrase sharing.
- Backup before broad writes/imports (`backup zip <store>`) and verify restore workflow periodically.
- Rotate sensitive passphrases/secrets on a regular cadence and after operator or host changes.
- Restrict filesystem access to store/config paths (`~/.local/share/vaultline`, `~/.config/vaultline`).
- After package upgrades/restarts, verify health and seal state explicitly (`vaultline health`).

## Developer references

- [Developer ramp-up](developer.md)
