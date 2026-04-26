# CLI

Most day-to-day vaultline operations are initiated through the CLI.

## Table of contents

- [Global flags](#global-flags)
- [Top-level commands](#top-level-commands)
- [Secret commands](#secret-commands)
- [Store commands](#store-commands)
- [Daemon listener commands](#daemon-listener-commands)
- [Completion](#completion)
- [Diagnostics](#diagnostics)
- [Related pages](#related-pages)

## Global flags

- `--addr 127.0.0.1:8428` — daemon address (default loopback)
- `--output json|text|raw|eval-export|eval-set` — output format (eval-* applies to `secret get`)

## Top-level commands

- `version` — prints CLI version only.
- `health` — prints daemon and store status.
- `seal` — seals the default store.
- `unseal` — unseals the default store.
- `daemon-stop` — asks daemon process to stop.
- `daemon ...` — manages extra listeners and allow rules.
- `store ...` — manages named stores and their seal lifecycle.
- `secret ...` — manages secret CRUD/search/transfer operations.
- `import ...` — imports data from external sources (`bitwarden`, `zip`).
- `export ...` — exports store data (`zip`).
- `backup ...` — creates full-store backups.
- `restore ...` — restores full-store backups.
- `completion ...` — prints shell completion scripts.

Detailed backup/restore workflow is documented on [Backup & restore](backup-restore.md).

## Secret commands

- `secret set <store:key> [--value|--file|--stdin] [--twice]` — create/update one secret.
- `secret get <store:key> [VAR] [--out path] [--output text|raw|json|eval-export|eval-set]` — read one secret.
- `secret delete <store:key>` — delete one secret.
- `secret list [store:] [--output json] [--raw]` — list store keys.
- `secret glob <store-glob:key-glob> [--raw]` — search keys by wildcard.
- `secret delete-prefix <store:prefix.> [--dry-run] [--yes]` — delete key ranges by prefix.
- `secret copy [oldstore:]oldname [newstore:]newname [--force]` — copy secret between keys/stores.
- `secret move [oldstore:]oldname [newstore:]newname [--force]` — move secret between keys/stores.

## Store commands

- `store add <name> <path>` — register an existing store path.
- `store init <name> [path] [--prompt-passphrase] [--remember-passphrase]` — create/register a new store.
- `store list [--raw]` — list configured stores and status.
- `store show <name>` — show one store record.
- `store unseal <name> [--prompt-passphrase] [--remember-passphrase|--transient] [--from-secret store:key]` — unseal named store.
- `store seal <name> [--keep-keys]` — seal named store.
- `store delete|remove|rm <name>` — remove store registration.

## Daemon listener commands

- `daemon bind <addr>` — add extra listener.
- `daemon list-binds` — list extra listeners.
- `daemon unbind <addr>` — remove listener and its allow rules.
- `daemon allow <addr> <cidr-or-ip>` — allow one source for one listener.
- `daemon list-allows <addr>` — list listener allow rules.
- `daemon unallow <addr> <cidr-or-ip>` — remove one allow rule.

## Completion

- `vaultline completion bash` — emits bash completion.
- `vaultline completion zsh` — emits zsh completion.

Completion supports both `vaultline` and `vl`, including dynamic suggestions for stores, listener addresses, allow rules, and qualified keys.

## Secret to environment

For shell usage, `secret get` can emit shell code for `eval`:

```bash
eval "$(vaultline --output eval-export secret get mystore:token TOKEN)"
eval "$(vaultline --output eval-set secret get mystore:token TOKEN)"
```

- `--output eval-export` prints `VAR='...'; export VAR;` (ssh-agent style).
- `--output eval-set` prints `VAR='...';` without export.
- In both eval modes, `VAR` is required as positional argument.

## Diagnostics

- `vaultline health`
- `curl http://127.0.0.1:8428/`

CLI exits non-zero when daemon is unavailable, target store is sealed, or operations fail.

## Related pages

- [Daemon & API](daemon.md)
- [Store model](store-model.md)
- [Backup & restore](backup-restore.md)
- [Bitwarden import](bitwarden.md)
- [Runbook (Project Workflow)](runbook.md)
