# vaultline CLI

`vaultline` is a localhost-only client for the vaultline daemon. It refuses to connect to non-loopback addresses unless rebuilt with custom flags, so keep the daemon on the same host.

## Global flags
- `--addr 127.0.0.1:8428` — daemon address (default loopback)
- `--output json|text|raw` — formatting used by commands that print responses

## Example session
```
$ export VAULTLINE_PASSPHRASE=correct-horse
$ go run ./cmd/vaultline daemon --store-dir .testrun/store
vaultline listening on 127.0.0.1:8428

$ go run ./cmd/vaultline --addr 127.0.0.1:8428 health
sealed=false status=ok

$ echo "abcd1234" | go run ./cmd/vaultline --addr 127.0.0.1:8428 \
      secret put --name infra.db-password --stdin
secret stored (version=2fbd6a7a4e)

$ go run ./cmd/vaultline --addr 127.0.0.1:8428 \
      secret get --name infra.db-password --raw
abcd1234

$ go run ./cmd/vaultline --addr 127.0.0.1:8428 \
      secret delete --name infra.db-password
secret removed

$ go run ./cmd/vaultline --addr 127.0.0.1:8428 \
      secrets list
infra.db-password
```

## Secret commands
Keys must be lowercase and may include `.` or `-` to express hierarchy (e.g., `app.payments.api-key`). The CLI simply proxies to the REST API:
- `secret put --name <key> [--value|--file|--stdin]` (omit all input flags to type the secret interactively when running in a TTY; input is masked)
- `secret get --name <key> [--out path] [--output raw|json]`
- `secret delete --name <key>`
- `secrets list [--output json]` — lists stored keys (text output by default)
- `secrets set|get|delete …` — aliases for the `secret` commands

## Diagnostics
- `vaultline health` — prints sealed state, status, and version
- `curl http://127.0.0.1:8428/` — returns a tiny HTML dashboard that lists a sample of stored keys when unsealed

The CLI exits non-zero when the daemon is sealed, missing, or rejects a request (e.g. invalid key names), which makes it safe to script.
