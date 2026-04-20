# Releasing vaultline

Release flow is `develop -> release`.

## Short checklist

1. Prepare release artifacts and notes on `develop`:

   ```bash
   ./scripts/prepare-release.sh <X.Y.Z>
   ```

2. Review and commit generated changes on `develop`.
3. Push `develop`.
4. Publish `release` branch:

   ```bash
   ./scripts/release.sh
   ```

5. GitHub Actions builds artifacts, tags `vX.Y.Z`, publishes the release, and deploys Pages.

## Important

- Do not bump versions manually on `release`; always prepare on `develop` first.
- `prepare-release.sh` handles version bump + release-note/data updates.

## Full documentation

- Runbook (project workflow): `docs/runbook.md`
- Developer ramp-up: `docs/developer.md`
- Site/docs build details: `README.md`
