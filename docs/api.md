# REST API

Base URL: `http://127.0.0.1:8428/api/v1`

vaultline exposes a minimal JSON API. `default` remains the default store, but additional named stores live under their own paths and have independent seal state.

## `GET /health`
Returns daemon status plus all known stores:
```json
{
  "status": "ok",
  "version": "0.2.x",
  "default_store": "default",
  "stores": [
    {
      "name": "default",
      "path": "/home/micwin/.local/share/vaultline/stores/default",
      "default": true,
      "available": true,
      "sealed": false,
      "has_key": true
    },
    {
      "name": "project-a",
      "path": "/srv/project-a",
      "default": false,
      "available": false,
      "sealed": false,
      "has_key": false,
      "error": "vaultline: store unavailable: missing store metadata at /srv/project-a"
    }
  ]
}
```
A broken external store does **not** stop the daemon as long as `default` is healthy.

## Default-store compatibility routes
These keep existing clients working and always target `default`:
- `POST /unseal`
- `POST /seal`
- `GET /secrets`
- `PUT /secrets/{name}`
- `GET /secrets/{name}`
- `DELETE /secrets/{name}`

## Named-store routes
### `GET /stores`
Lists all configured stores with path, availability, seal state, and whether a remembered passphrase exists.

### `POST /stores`
Creates or registers a store.

Request:
```json
{
  "name": "project-a",
  "path": "/srv/project-a",
  "initialize": true
}
```

When `initialize=true`, the daemon creates the store, generates a random unseal key, stores that key in the registry config, unseals the new store immediately, and returns it once:
```json
{
  "name": "project-a",
  "path": "/srv/project-a",
  "default": false,
  "available": true,
  "sealed": false,
  "has_key": true,
  "passphrase": "<printed-once>"
}
```

### `GET /stores/{store}`
Returns one store entry with the same fields shown in `GET /stores`.

### `POST /stores/{store}/unseal`
```json
{ "passphrase": "string" }
```
If the request carries an empty passphrase, the daemon first tries any remembered key already stored in `stores.json`. Returns:
```json
{ "store": "project-a", "sealed": false }
```

### `POST /stores/{store}/seal`
Request body:
```json
{ "keep_keys": false }
```
By default, sealing removes the remembered passphrase from the registry config. Set `keep_keys=true` to keep it.

### `GET /stores/{store}/secrets`
Lists keys for the chosen store:
```json
{
  "keys": [
    {
      "name": "infra.db-password",
      "version": "abc123",
      "updated_at": "2026-03-25T15:37:01Z"
    }
  ]
}
```

### `PUT /stores/{store}/secrets/{name}`
- `name` must match `^[\p{Ll}\p{Nd}@.-]+$`
- Body: `{ "value": "base64" }`
- Response: `{ "version": "hex" }`

### `GET /stores/{store}/secrets/{name}`
Returns `{ "value": "base64", "version": "hex" }`.

### `DELETE /stores/{store}/secrets/{name}`
Deletes the secret and returns `204 No Content` on success.

## Errors
Errors follow this shape:
```json
{ "error": "CODE", "message": "human readable description" }
```
Common codes:
- `SEALED` — target store is locked (`409`)
- `STORE_NOT_FOUND` — unknown store (`404`)
- `STORE_UNAVAILABLE` — configured store is missing or broken (`503`)
- `INVALID_IDENTIFIER` — key violated the naming rules (only lowercase letters incl. umlauts, digits, `@`, `.`, `-`) (`400`)
- `NOT_FOUND` — unknown key (`404`)
- `STORE_ERROR` — unexpected failure (`500`)
