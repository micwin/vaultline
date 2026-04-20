# Backup & restore

Operational guidance for creating and restoring full-store archives.

## Table of contents

- [Overview](#overview)
- [Create backups](#create-backups)
- [Restore backups](#restore-backups)
- [Recommended workflow](#recommended-workflow)
- [Failure handling](#failure-handling)

## Overview

Vaultline provides two equivalent archive workflows:

- `backup zip <store> [zip-file]` / `restore zip <zip-file> <store> [--overwrite]`
- `export zip <store> [zip-file]` / `import zip <zip-file> <store>`

`backup/restore` are the preferred operator-facing names for runbooks.

## Create backups

Create archive with explicit filename:

```bash
vaultline backup zip mystore ./mystore-backup.zip
```

Create archive with auto filename (`<store>-YYYY-MM-DD.zip`):

```bash
vaultline backup zip mystore
```

## Restore backups

Restore into store name:

```bash
vaultline restore zip ./mystore-backup.zip mystore
```

Overwrite existing target store explicitly:

```bash
vaultline restore zip ./mystore-backup.zip mystore --overwrite
```

## Recommended workflow

1. Seal/non-critical write pause for target store if possible.
2. Create backup before broad writes/imports.
3. Verify backup file integrity and storage location.
4. Test restore procedure periodically on a non-production store.
5. Keep at least one known-good archive outside the primary host.

## Failure handling

- If restore fails midway, inspect daemon logs and store status first.
- Re-run restore only with explicit `--overwrite` when replacement is intended.
- Keep previous backup archive until post-restore verification is complete.
