# vaultline CLI

`vaultline` is a localhost-only client for the vaultline daemon. It refuses to connect to non-loopback addresses unless rebuilt with custom flags, so keep the daemon on the same host.

## Global flags
- `--addr 127.0.0.1:8428` — daemon address (default loopback)
- `--output json|text|raw` — formatting used by commands that print responses

## Import commands
- `import bitwarden --all [--prefix PREFIX] [--store default] [--dry-run] [--add-missing-keys] [--overwrite-existing-keys]`
- `import bitwarden --item <name-or-id> [--prefix PREFIX] [--store default] [--dry-run] [--add-missing-keys] [--overwrite-existing-keys]`
- `import zip <zip-file> <store>`
- requires the official `bw` CLI plus an unlocked Bitwarden session (`BW_SESSION`)
- imports login credentials, URIs, TOTP, notes, and custom fields into Vaultline keys such as `project-a:github.password` or `project-a:bitwarden.github.password` when `--prefix bitwarden` is used
- Bitwarden prefix, folder names, item names, and custom field names are normalized only lightly: uppercase becomes lowercase, whitespace is removed, `_` becomes `-`, and `/`, `:`, `,`, `!`, `(`, `)` become `.`, and repeated `.` collapse into one. Any other illegal characters remain untouched; affected keys are reported and skipped so the source item, folder, or prefix can be fixed explicitly.
- imports are additive by default (`--add-missing-keys=true`, `--overwrite-existing-keys=false`); use `--overwrite-existing-keys` to refresh already imported keys

## Export commands
- `export zip <store> [zip-file]` — archive a full store directory into a zip file; without a filename, Vaultline writes `<store>-YYYY-MM-DD.zip` in the current directory
- `import zip <zip-file> <store>` — restore a zip archive into a store path (creates the store entry if needed)

## Shell completion
- `vaultline completion bash` — prints a bash completion script
- `vaultline completion zsh` — prints a zsh completion script
- the generated completion hooks both `vaultline` and `vl`
- completion is dynamic for:
  - configured store names
  - configured daemon bind addresses
  - configured allow rules for `daemon unallow`
- subcommands still support their own `--help`, for example `vaultline store init --help`

## Store model
- `default` is the default store.
- Additional stores are addressed by prefixing keys with `store:` (for example `project-a:infra.db-password`).
- Each store has its own path, `.master_salt`, seal state, and passphrase.
- `store init` generates a random unseal key, prints it once, stores it in the registry config, and leaves the store immediately unsealed.

## Example session
```text
$ export VAULTLINE_PASSPHRASE=correct-horse
$ go run ./cmd/vaultline daemon --store-dir .testrun/store
vaultline listening on 127.0.0.1:8428

$ go run ./cmd/vaultline --addr 127.0.0.1:8428 health
status=ok default_store=default
default  available=true  sealed=false  has_key=true

$ go run ./cmd/vaultline --addr 127.0.0.1:8428 store init project-a .testrun/project-a
store project-a ready
unseal key: <printed-once>
stored in config and immediately unsealed

$ echo "abcd1234" | go run ./cmd/vaultline --addr 127.0.0.1:8428 \
      secret set project-a:infra.db-password --stdin
secret stored in project-a (version=2fbd6a7a4e)

$ go run ./cmd/vaultline --addr 127.0.0.1:8428 \
      secret get project-a:infra.db-password --output raw
abcd1234

$ go run ./cmd/vaultline --addr 127.0.0.1:8428 \
      store seal project-a --keep-keys
store project-a sealed

$ go run ./cmd/vaultline --addr 127.0.0.1:8428 \
      store unseal project-a
store project-a unsealed
```

## Secret commands
Keys must be lowercase and may include umlauts, digits, `@`, `.` or `-` to express hierarchy (e.g. `mötor-1.api-key@prod`). Prefixing with `store:` selects a non-default store.

- `secret set <store:key> [--value|--file|--stdin] [--twice]`
  - omit all input flags to type the secret interactively when running in a TTY; input is masked
  - add `--twice` together with interactive `--stdin` to require a matching confirmation prompt
  - omit the prefix to target `default`
- `secret get <store:key> [--out path] [--output raw|json]`
- `secret delete <store:key>`
- `secret delete-prefix <store:prefix.> [--dry-run] [--yes]`
- `secret glob <store-glob:key-glob> [--raw]`
- `secret list [store:] [--output json] [--raw]`
  - text output shows `store:key`, update timestamp, and version
  - `--raw` prints only matching key names, one per line
  - `delete-prefix` only deletes keys below a dotted prefix and requires `--yes` unless used with `--dry-run`
  - `glob` searches qualified keys with shell-style wildcards, e.g. `bitw*:*.zf.*test*`

## Store commands
- `store add <name> <path>` — register an existing store
- `store init <name> [path] [--prompt-passphrase] [--remember-passphrase]` — create, register, and immediately unseal a new store; when omitted, the path defaults next to the default store. With `--prompt-passphrase`, the CLI asks twice for a custom store passphrase instead of generating one and only echoes it back in masked form. Prompted passphrases are not stored unless `--remember-passphrase` is also set.
- `store list [--raw]` — show every configured store plus availability/seal state; `--raw` prints only store names
- `store show <name>` — dump one store entry as JSON
- `store unseal <name> [--prompt-passphrase] [--remember-passphrase|--transient]` — first tries any remembered passphrase; prompts only if none is stored unless `--prompt-passphrase` forces a fresh prompt
- `store seal <name> [--keep-keys]` — seal the store; remembered passphrases are removed unless `--keep-keys` is specified
- `store delete|remove|rm <name>` — remove a store from the registry without deleting files on disk
- every command and subcommand supports `--help`, for example `vaultline store init --help` or `vaultline secret set --help`

## Daemon listener commands
- `daemon bind <addr>` — add an extra listener (for example `0.0.0.0:8384`)
- `daemon list-binds` — list configured extra listeners and whether they are blocked/allowlisted
- `daemon unbind <addr>` — remove a listener and all of its allow rules
- `daemon allow <addr> <cidr-or-ip>` — allow a remote IP/CIDR for one listener
- `daemon list-allows <addr>` — list allow rules for one listener
- `daemon unallow <addr> <cidr-or-ip>` — remove one allow rule
- loopback stays implicitly available for the CLI and cannot be managed through these commands
- in the container image, these commands are intended to be run through `docker exec vaultline vaultline ...`; remote clients cannot access bind/allow management over HTTP

## Diagnostics
- `vaultline health` — prints daemon status, default store, the status of every configured store, and all extra daemon binds plus their allow counts/state
- `curl http://127.0.0.1:8428/` — returns a tiny HTML dashboard listing every known store and whether it is sealed, unsealed, or unavailable

The CLI exits non-zero when the daemon is missing, the target store is sealed, or a configured store is unavailable/broken.
