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
