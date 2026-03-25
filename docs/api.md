# REST API

Base URL: `http://127.0.0.1:8428/api/v1`

vaultline exposes a minimal JSON API. All routes require the daemon to be unsealed first.

## `GET /health`
```json
{
  "sealed": false,
  "status": "ok",
  "version": "0.1.8"
}
```

## `POST /unseal`
```
{ "passphrase": "string" }
```
Unlocks the store using the provided passphrase. Returns `{ "sealed": false }` on success.

## `POST /seal`
Re-locks the store. Subsequent secret operations will fail with `409 SEALED` until `POST /unseal` runs again.

## `PUT /secrets/{name}`
- `name` must match `^[a-z.-]+$`
- Body: `{ "value": "base64" }`
- Response: `{ "version": "hex" }`

## `GET /secrets/{name}`
Returns `{ "value": "base64", "version": "hex" }`. `404 NOT_FOUND` if the key is missing.

## `DELETE /secrets/{name}`
Deletes the secret. Returns `204 No Content` on success.

## Errors
Errors follow this shape:
```
{ "error": "CODE", "message": "human readable description" }
```
Common codes:
- `SEALED` — daemon is locked (`409`)
- `INVALID_IDENTIFIER` — key violated the naming rules (`400`)
- `NOT_FOUND` — unknown key (`404`)
- `STORE_ERROR` — unexpected failure (`500`)
