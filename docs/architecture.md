# vaultline architecture

## Goals recap
- Git-safe persistence: every secret is a separate encrypted file to minimize merge conflicts and reveal history clearly.
- Manual unlock: daemon remains sealed until a passphrase (interactive or env) derives the master key.
- Subspaces: logical partitions (e.g., `prod`, `staging`, `contractor`) with optional dedicated passphrases.
- CLI + REST parity: every API call has an equivalent CLI command so humans can script locally while automation hits the HTTP surface.
- Import/export: allow selective replication of secrets across hosts without leaking unrelated ones.
- MFA: opt-in TOTP enforcement for either subspaces or single secrets.

## Storage layout
```
store/
  manifest.json            # encrypted manifest describing spaces + policies
  spaces/
    default/
      namespaceA/
        secretA.vlx        # AEAD payload (ciphertext + nonce + tag)
        secretA.meta.json  # encrypted metadata (labels, version, history)
      namespaceB/
        ...
    infra/
      secrets/...          # same layout but optional independent passphrase
```
- Filenames are ASCII (`[a-z0-9-_]+`).
- `.vlx` files contain `{version, nonce, salt, ciphertext}` encoded as CBOR to avoid JSON parsing overhead while remaining deterministic.
- Metadata keeps timestamps, checksum of plaintext, optional labels, TOTP requirements, and ACL markers. Metadata is encrypted with the same key as the payload to hide labels.

## Key derivation
1. Operator supplies unlock passphrase via prompt or `VAULTLINE_PASSPHRASE`.
2. Derive master key `K0 = Argon2id(passphrase, salt_master)` where `salt_master` is stored (plaintext) in `store/.vaultline`. Strength: `t=4`, `m=512MB`, `p=2` by default, configurable.
3. Each secret uses Envelope Encryption: `Ksecret = HKDF(Kspace || namespace || secretId)` with per-secret salt stored next to ciphertext.
4. Subspace-specific passphrase: `Kspace = Argon2id(subspace_passphrase, salt_space)`. The subspace descriptor (which states whether it has its own passphrase) is encrypted with `K0` so only an unlocked daemon knows the requirement.

## Unlock / runtime state
- `vaultlined` keeps only derived keys in memory (`sealedbox`), wiped upon shutdown.
- REST API rejects requests until `/unseal` receives passphrase; CLI mirrors this by refusing to send secret material if daemon reports `sealed: true`.
- Optional `vaultline-agent` systemd service prompts for passphrase at boot.

## REST API surface (high level)
- `POST /unseal` — provide passphrase or `subspace_passphrase` payloads.
- `GET /health` — sealed status and version.
- `GET /spaces`, `POST /spaces` — manage subspaces + policies.
- `GET/PUT/DELETE /spaces/{space}/namespaces/{ns}/secrets/{id}` — CRUD with optimistic locking via `If-Match` headers.
- `POST /spaces/{space}/export` — stream tar of selected secrets.
- `POST /spaces/{space}/import` — upload tar, verify signatures, optional dry-run.
- `POST /totp/enroll` — create TOTP secret (encrypted); returns provisioning URI for authenticator apps.
- `POST /totp/verify` — verify code; optionally attach to API tokens.

Detailed request/response examples are listed in `docs/api.md`.

## CLI considerations
- `vaultlinectl` defaults to `http://127.0.0.1:8428` and refuses non-loopback hosts unless `--unsafe-remote` flag is acknowledged.
- Provides `vaultlinectl login` (prompts for passphrase, calls `/unseal`), `vaultlinectl secret put/get/delete`, `vaultlinectl space export/import`, `vaultlinectl totp enroll/verify`.
- Supports piping: `cat .env | vaultlinectl secret put default app env --stdin`.

## Import/export format
- `tar` archive with deterministic ordering.
- Includes `MANIFEST.json` (plaintext) describing which secrets are included, their checksums, and required subspace passphrases (referenced by hash, not stored).
- Payload files remain encrypted; import simply copies files unless `--rekey` is passed, which decrypts + re-encrypts using local keys (requires matching subspace passphrases).

## MFA strategy
- Subspace policy can mandate TOTP. When enabled, every API call requiring secret plaintext must include `X-Vaultline-TOTP: 123456` header (validated server-side).
- CLI caches recent successful TOTP for 30s by default.
- TOTP secrets stored encrypted; provisioning URIs shown only once during enrollment.

## Tech stack
- Suggested implementation: Go 1.22 with chi/httprouter, age/ChaCha20-Poly1305 library, and fsnotify for change detection.
- Unit tests around storage, crypto, API handlers; integration tests spin up daemon + CLI in temp dirs.
