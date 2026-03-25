# vaultline REST API

All endpoints live under `/api/v1`. Requests require HTTPS in production; for local development the daemon can run on `127.0.0.1` without TLS.

## Authentication & headers
- `X-Vaultline-Session: <token>` — issued after `/auth/unseal` succeeds. Tokens expire after 24h or on daemon restart.
- `X-Vaultline-TOTP: 123456` — optional but required when the target subspace or secret enforces MFA.
- `If-Match: <version>` — optimistic locking for secret updates.

## Core endpoints

### POST /auth/unseal
Unlocks the daemon.
```json
{
  "passphrase": "...",      // required main passphrase or VAULTLINE_PASSPHRASE
  "subspaces": {
    "infra": "..."          // optional map of subspace-specific passphrases
  }
}
```
Responses:
- 200: `{ "sealed": false, "token": "..." }`
- 409: `{ "sealed": true, "missing_subspaces": ["infra"] }`

### GET /health
Returns sealed status and build metadata. Always 200.
```json
{
  "sealed": false,
  "version": "0.1.0",
  "uptime": 1234
}
```

### GET /spaces
Lists subspaces the session may access.
```json
{
  "spaces": [
    {"name": "default", "namespaces": ["app", "ops"], "totp": false},
    {"name": "infra", "namespaces": [], "totp": true}
  ]
}
```

### POST /spaces
Create subspace.
```json
{
  "name": "contractors",
  "passphrase": null,
  "namespaces": ["read-only"],
  "totp": true
}
```
- 201: `{ "name": "contractors" }`
- 409: space exists.

### Secret CRUD
`/spaces/{space}/namespaces/{ns}/secrets/{id}`
- `GET` returns `{ "value": "base64...", "version": "abc123", "labels": {...} }`.
- `PUT` accepts `{"value": "base64...", "labels": {...}, "ttl": 3600}`.
- `DELETE` removes file; returns 204.

### Export/import
`POST /spaces/{space}/export`
```json
{
  "namespaces": ["app"],
  "selectors": {"env": "prod"}
}
```
- Response: `application/octet-stream` tarball.

`POST /spaces/{space}/import?dry_run=true`
- Body: tarball. Response summarises actions (created, updated, skipped).

### TOTP
- `POST /totp/enroll` → `{ "provisioning_uri": "otpauth://totp/..." }`
- `POST /totp/verify` → `{ "valid": true }`
- When binding to a subspace: `POST /spaces/{space}/totp/bind` with `"secret_id": "..."` referencing stored TOTP secret.

## Error model
Errors follow:
```json
{
  "error": {
    "code": "SECRET_NOT_FOUND",
    "message": "...",
    "details": {...}
  }
}
```
Common codes: `SEALED`, `UNAUTHORIZED`, `TOTp_REQUIRED`, `CONFLICT`, `VALIDATION_FAILED`, `IMPORT_MISMATCH`.
