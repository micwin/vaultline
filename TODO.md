# TODO

- Nice to have: `vaultline store exec <store> -- <command ...>`.
  - Temporarily unseal a store for one command.
  - Optionally prompt for the passphrase without storing it.
  - Reseal the store automatically afterwards.

- Add `secret get --outfile` (or equivalent explicit file-only flag).
  - Prevent accidental binary output to terminal/stdout.
  - Keep current `--out` behavior for compatibility, but document safer default for binary payloads.
  - Add optional `--encrypt` mode for outfile writes (use available local tooling such as `openssl`, `age`, or configured command pipeline).
  - Surface the effective encryption method in CLI output so users can verify how data was protected.

- Add a CLI-only mode (no daemon required).
  - Allow core secret/store operations directly against local store/config paths.
  - Keep command behavior as close as possible to daemon-backed mode.
  - Document trade-offs (concurrency, locking, and remote access limitations).

- Improve secret-name completion for the final segment.
  - Current tab-completion can stop before suggesting the last key segment (for example `ai:....id`).
  - Ensure `secret` completion suggests full leaf candidates, not only intermediate dotted prefixes.
