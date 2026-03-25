# vaultline

vaultline is a Git-friendly secret store that blends the manual-unseal discipline of Vault with the per-file simplicity of git-backed tools. Each secret lives in its own encrypted file, the data directory can be safely committed, and a single unlock passphrase (typed interactively or delivered via `VAULTLINE_PASSPHRASE`) derives all encryption keys unless a subspace overrides them. A REST daemon exposes CRUD operations, import/export helpers sync subsets between hosts, and a localhost-only CLI (`vaultlinectl`) talks to the daemon via loopback sockets. Optional TOTP policies (Google Authenticator compatible) gate access to sensitive subspaces.

## Key guarantees
- **Per-secret files**: Secret payloads live under `store/<space>/<namespace>/<secret>.vlx`, allowing Git merges without binary blobs. Metadata sits alongside (`.meta.json`) and stays encrypted with the same key ladder.
- **Deterministic encryption**: Passphrase → Argon2id → master key. Subspaces can define `subspace.passphrase`, deriving independent keys while keeping their descriptors encrypted by the master key.
- **Manual unseal**: The daemon refuses to serve the REST API until a passphrase arrives via stdin, TTY prompt, or `VAULTLINE_PASSPHRASE`. Unlock status is stored only in memory; restarts require re-entry.
- **API + CLI**: REST speaks JSON over HTTP, while `vaultlinectl` wraps it with ergonomic commands (`vaultlinectl secret put`, `... get`, `... space export`). The CLI enforces `localhost` sockets to avoid remote hops.
- **Portability**: `vaultline export --space workspace --filter env=prod` bundles selected files plus metadata manifests into a tarball. Imports verify space checksums before merging.
- **MFA hooks**: Secrets or spaces can demand TOTP. The daemon validates tokens using RFC 6238; shared secrets themselves live encrypted so Git never sees them in the clear.

## Repository layout
```
vaultline/
  README.md
  docs/
    architecture.md        # detailed design & storage layout
    api.md                  # REST surface + error model
    cli.md                  # vaultlinectl UX & examples
  cmd/
    vaultlined/            # daemon entrypoint (to be implemented)
    vaultlinectl/          # CLI entrypoint (to be implemented)
  pkg/
    storage/               # file-per-secret backend
    crypto/                # Argon2id key derivation + AEAD wrappers
    api/                   # HTTP handlers, subspace auth, TOTP policy
    cli/                   # shared logic between daemon + CLI
  store/                   # sample test fixtures (empty in repo)
```

## Next steps
1. Finalise the storage format (meta schema, filename rules) in `docs/architecture.md`.
2. Define REST routes and CLI commands in their respective docs.
3. Scaffold the Go (or Rust) module so daemon + CLI share crypto/storage libraries.
4. Provide `docker-compose.yml` for local runs (daemon + optional reverse proxy) and document unlock / TOTP bootstrap flows.

## Development setup
- Install Go 1.22+, Docker, and supporting build tools on Debian/Ubuntu with `./scripts/bootstrap-debian.sh`. The script also installs Smokey (our test runner) for the current user by invoking `../smokey/install.sh`, so `smokey` is available on the `PATH`. Reopen the shell so the `docker` group membership takes effect.
- Run tests with `go test ./...` and execute the numbered suite via `smokey --tests-dir tests.d` (from inside `vaultline/`). Entries are sorted alphabetically; they may be single scripts (e.g., `020-api-health.sh`) or directories containing a lone executable (`000-setup/run.sh`, `999-teardown/run.sh`).
  - `000-setup/` creates `.testrun/`, boots `vaultlined` on `127.0.0.1:19428`, and writes connection details to `.testrun/env`. If setup exits with `SMOKEY_SKIP_CODE` (20) the runner marks the suite as failed, skips intermediate tests, yet still executes teardown.
  - `999-teardown/` always runs to stop the daemon and clean `.testrun/`, even when earlier steps fail.
  - Each test runs in a subshell with read-only env vars: `SMOKEY_TEST_ROOT`, `SMOKEY_TEST_SCRIPT`, optional `SMOKEY_TEST_DIR`, and `SMOKEY_SKIP_CODE`. Tests can also source `.testrun/env` for `VAULTLINE_TEST_ADDR`, `VAULTLINE_TEST_PASS`, and related metadata.
- Temporary fixtures or encrypted samples belong under `testdata/` so they can be referenced from unit tests without polluting the live `store/` directory.

## Running the daemon and CLI
1. `go run ./cmd/vaultline daemon --store-dir ./store` — starts the REST API on `127.0.0.1:8428`. If `--store-dir` is omitted, vaultline uses `$XDG_DATA_HOME/vaultline/store` or `~/.local/share/vaultline/store`. Provide `VAULTLINE_PASSPHRASE` to auto-unseal during startup or pass it via environment variables/service configuration. The first successful unseal determines the passphrase for the entire store; afterwards all unseal operations must use the same passphrase. When unsealed, visit `http://127.0.0.1:8428/` to see the current version, seal status, and a sample of spaces/namespaces/secrets (names only). Use `--seal-file /path/to/file` to enable file-based auto-unseal: vaultline will create the file (0600) on first run with a randomly generated passphrase and reuse it on subsequent starts. Keep the seal file outside the store directory and treat it like any other secret.
2. `go run ./cmd/vaultline --addr 127.0.0.1:8428 secret put --space default --namespace app --name api-key --value "$(openssl rand -hex 16)"`.
3. Fetch secrets with `go run ./cmd/vaultline --addr 127.0.0.1:8428 secret get --space default --namespace app --name api-key --out ./secret.txt`. The CLI is a thin REST client that requires a running daemon reachable via `--addr` and refuses to talk to non-loopback addresses unless you rebuild it with explicit overrides.

## Container image
- Build and run with Docker Compose: `VAULTLINE_PASSPHRASE=my-pass docker compose up --build`. Data persists under `./data/`.
- The multi-stage `Dockerfile` produces both `vaultlined` and `vaultlinectl` binaries; runtime image defaults to the non-root `vaultline` user and exposes 8428/TCP.
