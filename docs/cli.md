# vaultlinectl CLI

`vaultlinectl` is the human-friendly interface to the vaultline daemon. It refuses to connect to non-loopback addresses unless `--unsafe-remote` is passed with `--i-know-the-risks`.

## Global flags
- `--addr 127.0.0.1:8428` — daemon address (default loopback).
- `--token-file ~/.config/vaultline/session` — path storing session token.
- `--output json|table|raw` — response formatting.

## Login & unlock
```
$ vaultlinectl login
Enter passphrase: ********
Subspace infra passphrase (optional): ********
Unsealed ✓
```
- Reads passphrase from TTY; respects `VAULTLINE_PASSPHRASE` for automation.
- Stores session token locally with `0600` permissions.

## Secret commands
```
$ vaultlinectl secret put default app api-key \
    --from-env API_KEY

$ vaultlinectl secret get default app api-key --output raw
abcd1234

$ cat .env | vaultlinectl secret put default app env --stdin
```
- Supports `--labels env=prod,service=api`.
- `--version <etag>` enables optimistic concurrency.

## Space management
```
$ vaultlinectl space list
NAME       NAMESPACES     TOTP
default    app,ops        no
infra      core           yes

$ vaultlinectl space create contractors --totp --namespaces onboarding
```
- `space export <name> --namespaces app --selector env=prod > prod.tar`
- `space import <name> --file prod.tar --rekey` (prompts for subspace passphrase if required).

## MFA utilities
```
$ vaultlinectl totp enroll --label "vaultline infra"
otpauth://totp/vaultline:infra?...secret=ABCDEF...

$ vaultlinectl totp verify --code 123456
Code valid (expires in 23s)
```
- CLI caches last successful TOTP for 30 seconds per session to limit prompts during scripting.

## Diagnostics
- `vaultlinectl status` — prints sealed state, version, unlocked spaces.
- `vaultlinectl doctor` — checks file permissions, git cleanliness, and ensures `.gitignore` excludes temp files.

All commands exit non-zero on failure and echo REST error codes for scripting.
