# vaultline CLI

`vaultline` is a localhost-only client for the vaultline daemon. It refuses to connect to non-loopback addresses unless rebuilt with custom flags, so keep the daemon on the same host.

## Global flags
- `--addr 127.0.0.1:8428` — daemon address (default loopback)
- `--output json|text|raw` — formatting used by commands that print responses

## Store model
- `local` is the default store.
- Additional stores are addressed by prefixing keys with `store:` (for example `project-a:infra.db-password`).
- Each store has its own path, `.master_salt`, seal state, and passphrase.
- `store init` generates a random unseal key, prints it once, stores it in the registry config, and leaves the store immediately unsealed.

## Example session
```text
$ export VAULTLINE_PASSPHRASE=correct-horse
$ go run ./cmd/vaultline daemon --store-dir .testrun/store
vaultline listening on 127.0.0.1:8428

$ go run ./cmd/vaultline --addr 127.0.0.1:8428 health
status=ok default_store=local
local   available=true  sealed=false  has_key=true

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
Keys must be lowercase and may include `.` or `-` to express hierarchy (e.g. `app.payments.api-key`). Prefixing with `store:` selects a non-default store.

- `secret set <store:key> [--value|--file|--stdin]`
  - omit all input flags to type the secret interactively when running in a TTY; input is masked
  - omit the prefix to target `local`
- `secret get <store:key> [--out path] [--output raw|json]`
- `secret delete <store:key>`
- `secret list [store:] [--output json]`
  - text output shows `store:key`, update timestamp, and version

## Store commands
- `store add <name> <path>` — register an existing store
- `store init <name> <path>` — create, register, and immediately unseal a new store
- `store list` — show every configured store plus availability/seal state
- `store show <name>` — dump one store entry as JSON
- `store unseal <name>` — first tries any remembered passphrase; prompts only if none is stored
- `store seal <name> [--keep-keys]` — seal the store; remembered passphrases are removed unless `--keep-keys` is specified

## Diagnostics
- `vaultline health` — prints daemon status, default store, and the status of every configured store
- `curl http://127.0.0.1:8428/` — returns a tiny HTML dashboard listing every known store and whether it is sealed, unsealed, or unavailable

The CLI exits non-zero when the daemon is missing, the target store is sealed, or a configured store is unavailable/broken.
