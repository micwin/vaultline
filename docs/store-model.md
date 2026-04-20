# Store model

Vaultline supports one default store plus any number of named stores.

## Key addressing

- Default store key: `infra.db-password`
- Named store key: `project-a:infra.db-password`

The `store:` prefix selects the store. The key segment remains the on-disk secret name.

## Store isolation

Each store has independent:

- `.master_salt`
- passphrase
- seal state
- secret files

This means one store can be sealed/unsealed without affecting others.

## Remembered passphrases

- `store init` can persist generated passphrases in the local registry.
- `store seal` removes remembered keys unless `--keep-keys` is used.
- `store unseal` first tries remembered material, then explicit input/prompt.

## Storage paths

See [Architecture](architecture.md) for path-level details (`stores.json`, `secrets/*.vlx`, salts).
