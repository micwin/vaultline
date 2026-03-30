# Architecture

## Storage layout
- Default local store: `~/.local/share/vaultline/store` (configurable via `--store-dir`)
- Named-store registry: `~/.config/vaultline/stores.json` (configurable via `--config-file`)
- Each store has its own root:
  - `.master_salt` (base64 encoded)
  - `secrets/<name>.vlx`
- Secret names remain lowercase and may include `.` or `-`
- Fully qualified keys use the form `store:key`; only the prefix selects the store, the secret filename stays `key.vlx`

## Multi-store model
- `local` is always registered and points at the daemon's primary store directory.
- Additional stores are registered in `stores.json` with their own paths and (optionally) remembered passphrases.
- Every store is an independent `storage.Store`, meaning:
  - separate salt
  - separate passphrase
  - separate seal state
  - separate files on disk
- Broken or missing external stores do not block daemon startup as long as `local` is healthy. They are surfaced as `available=false` in health/status output.

## Key derivation
For each store independently:
1. User supplies passphrase via CLI, `VAULTLINE_PASSPHRASE`, seal file, or a remembered key in `stores.json`
2. Argon2id derives a 32-byte master key using that store's `.master_salt`
3. Each secret name feeds `HMAC-SHA256(master, name)`; the result seeds XChaCha20-Poly1305
4. Sealing zeroes the in-memory master key
5. Unless `--keep-keys` is used, sealing also removes any remembered passphrase from the registry config

## API surface
Compatibility routes still target `local`:
- `GET /api/v1/health`
- `POST /api/v1/unseal`
- `POST /api/v1/seal`
- `PUT/GET/DELETE /api/v1/secrets/{name}`

Named-store routes:
- `GET /api/v1/stores`
- `POST /api/v1/stores`
- `GET /api/v1/stores/{store}`
- `POST /api/v1/stores/{store}/unseal`
- `POST /api/v1/stores/{store}/seal`
- `PUT/GET/DELETE /api/v1/stores/{store}/secrets/{name}`
- `GET /api/v1/stores/{store}/secrets`

## Remembered passphrases
- `store init` generates a random passphrase, stores it in the registry config, and leaves the new store unsealed.
- `store unseal <name>` and `vaultline unseal` first try the remembered passphrase before prompting.
- `seal` removes remembered passphrases unless `--keep-keys` is requested.

## Dashboard and health
`GET /` renders a static HTML page with version plus one line per configured store (`sealed`, `unsealed`, or `unavailable`).

`GET /api/v1/health` returns the same status in JSON and is intended to stay readable even when one project-specific store is missing.

## Testing
Smokey suites live under `tests.d/` and spin up the daemon in `.testrun/`. They now cover both the default `local` store and an additional named store, including `store init`, `store unseal`, and prefixed `secret get/set/list` flows.
