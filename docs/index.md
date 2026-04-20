# vaultline docs

This documentation complements the main release site and focuses on hands-on operation.

## Table of contents

- [Start here](#start-here)
- [Typical flow](#typical-flow)
- [Operational checklist](#operational-checklist)
- [Related links](#related-links)

## Start here

- Use [CLI](cli.md) for command usage and completion behavior.
- Use [Daemon & API](daemon.md) for process behavior, service integration, and endpoint details.
- Use [Store model](store-model.md) for multi-store semantics and storage layout.
- Use [Backup & restore](backup-restore.md) for archive and recovery operations.
- Use [Runbook (Project Workflow)](runbook.md) for release/versioning workflow and helper scripts.

## Typical flow

1. **Start and verify daemon readiness** (`vaultline daemon`, `vaultline health`) so API and store state are visible before any secret operation.
   See: [Daemon & API](daemon.md).
2. **Initialize/register stores** (`store init`, `store add`) to define store boundaries and passphrase handling early.
   See: [Store model](store-model.md) and [CLI](cli.md).
3. **Unseal only required stores** (`store unseal`, optionally `--from-secret`) to minimize exposure and keep per-store access explicit.
   See: [Daemon & API](daemon.md#unseal-behavior).
4. **Execute secret lifecycle commands** (`secret set/get/list/glob/copy/move/delete`) as the primary operational interface.
   See: [CLI](cli.md#secret-commands).
5. **Run backup/export and reseal workflow** before and after sensitive changes.
   See: [Backup & restore](backup-restore.md) and [CLI](cli.md).

## Operational checklist

- Confirm daemon health and store status with `vaultline health`.
  See: [Daemon & API](daemon.md).
- Unseal only the stores required for the current task.
  See: [Daemon & API](daemon.md#unseal-behavior).
- Snapshot/backup before broad updates.
  See: [Backup & restore](backup-restore.md).
- Perform secret changes via CLI commands.
  See: [CLI](cli.md#secret-commands).
- Reseal stores according to your operating policy.
  See: [Store model](store-model.md) and [Daemon & API](daemon.md).

## Related links

- Main site overview: `/vaultline/`
- Downloads and release notes: `/vaultline/downloads/`
- Repository: <https://github.com/micwin/vaultline>
