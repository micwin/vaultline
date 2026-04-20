# Bitwarden import

Vaultline imports Bitwarden data through the official `bw` CLI.

## Requirements

- `bw` is installed and available on `PATH`
- `BW_SESSION` is set for an unlocked Bitwarden session

## Commands

```bash
vaultline import bitwarden --all --store default
vaultline import bitwarden --item "GitHub" --store default
```

Useful flags:

- `--prefix <prefix>`
- `--dry-run`
- `--add-missing-keys`
- `--overwrite-existing-keys`

## Mapping notes

- Imports include usernames, passwords, notes, URIs, TOTP, and custom fields.
- Key normalization is intentionally conservative.
- Invalid generated keys are reported and skipped so source data can be corrected explicitly.

## Recommended workflow

1. Run with `--dry-run`.
2. Review skipped/invalid key reports.
3. Import without `--dry-run` once key mapping looks correct.
