# Architecture

## Storage layout
- Store root: `~/.local/share/vaultline/store` (configurable via `--store-dir`)
- Master salt: `.master_salt` (base64 encoded)
- Secrets: `secrets/<name>.vlx`
  - `<name>` is lowercase and may include `.` or `-`
  - File payload is a JSON envelope containing nonce, ciphertext, version, and `created_at`

## Key derivation
1. User supplies passphrase via CLI/`VAULTLINE_PASSPHRASE`
2. Argon2id derives a 32-byte master key using the master salt
3. Each secret name feeds `HMAC-SHA256(master, name)`; the result seeds XChaCha20-Poly1305
4. The daemon keeps keys only in memory; `POST /seal` zeroes the master key

## API surface
- `GET /api/v1/health` — health + sealed status
- `POST /api/v1/unseal` — `{ passphrase }`
- `POST /api/v1/seal`
- `PUT/GET/DELETE /api/v1/secrets/{name}` — CRUD operations on a single key

## Dashboard
`GET /` renders a static HTML page with version, seal status, and up to 20 stored keys for quick inspection. No values are returned.

## Testing
Smokey suites live under `tests.d/` and spin up the daemon in `.testrun/`. They ensure secrets survive restarts and verify key validation rules.
