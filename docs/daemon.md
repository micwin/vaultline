# Daemon & API

The daemon is the stateful process behind all CLI operations and the HTTP API surface.

## Table of contents

- [Start and process model](#start-and-process-model)
- [Systemd/deb integration](#systemddeb-integration)
- [Configuration and paths](#configuration-and-paths)
- [Unseal behavior](#unseal-behavior)
- [HTTP API](#http-api)
- [Error model](#error-model)

## Start and process model

```bash
vaultline daemon --store-dir ./store
```

Default bind address is `127.0.0.1:8428`.

Key flags:

- `--addr HOST:PORT` — daemon listener (default loopback)
- `--store-dir DIR` — default store path
- `--config-file PATH` — named-store registry (`stores.json`)
- `--daemon-config-file PATH` — daemon bind/allow config (`daemon.json`)
- `--seal-file PATH` — startup passphrase source for default store

## Systemd/deb integration

Systemd unit template in source:

- [`systemd/vaultline@.service`](https://github.com/micwin/vaultline/blob/develop/systemd/vaultline@.service)

The Debian package manages service instances during upgrade:

- `prerm` records current `vaultline@*.service` state, then stops/disables units.
- `postinst` restores enable/start state and reloads systemd.

Because daemon processes are restarted during package upgrades, stores that are not automatically unsealed may come back sealed after update.

## Configuration and paths

From the packaged systemd service template, runtime paths are per-user under home directories:

- Default store path: `/home/<user>/.local/share/vaultline/stores/default`
- Seal file: `/home/<user>/.config/vaultline/seal`
- Store registry: `/home/<user>/.config/vaultline/stores.json`
- Daemon bind/allow config: `/home/<user>/.config/vaultline/daemon.json`

So config is **not** in `/etc/vaultline` with current packaging.

## Unseal behavior

- `VAULTLINE_PASSPHRASE` applies to the **default store only**.
- Named stores are unsealed via `vaultline store unseal <name> ...`.
- `store unseal --from-secret <store:key>` reads unseal material from another store.
- If no explicit passphrase is supplied, daemon unseal tries remembered passphrase material first.

## HTTP API

### `GET /api/v1/health`

Returns daemon and store status summary.

### `GET /api/v1/stores`

Lists configured stores with path, availability, seal state, and remembered-key state.

### `POST /api/v1/stores`

Create/register a named store (`initialize=true` creates and unseals immediately).

### `GET /api/v1/stores/{store}`

Returns one store entry.

### `POST /api/v1/stores/{store}/unseal`

Request body:

```json
{ "passphrase": "...", "remember_passphrase": false }
```

### `POST /api/v1/stores/{store}/seal`

Request body:

```json
{ "keep_keys": false }
```

### `GET /api/v1/stores/{store}/secrets`

List all keys in one store.

### `PUT /api/v1/stores/{store}/secrets/{name}`

Store/update one secret (`{ "value": "base64" }`).

### `GET /api/v1/stores/{store}/secrets/{name}`

Fetch one secret (`{ "value": "base64", "version": "..." }`).

### `DELETE /api/v1/stores/{store}/secrets/{name}`

Delete one secret.

## Error model

Errors follow:

```json
{ "error": "CODE", "message": "description" }
```

Common codes include:

- `SEALED`
- `STORE_NOT_FOUND`
- `STORE_UNAVAILABLE`
- `INVALID_IDENTIFIER`
- `NOT_FOUND`
- `STORE_ERROR`
