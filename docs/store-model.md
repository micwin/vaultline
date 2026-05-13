# Store model

Vaultline supports one default store plus any number of named stores.

## Key addressing

- Default store key: `infra.db-password`
- Named store key: `project-a:infra.db-password`

The `store:` prefix selects the store. The key segment remains the on-disk secret name.

## Store isolation

Each store has independent:

- `.master_salt`
- `.verifier`
- passphrase
- seal state
- secret files

This means one store can be sealed/unsealed without affecting others.

## Passphrase verification

Unseal verifies the supplied passphrase before marking a store unsealed. New stores write an encrypted `.verifier` file during first unseal. Existing stores without `.verifier` are migrated lazily: Vaultline first proves the candidate key by decrypting an existing secret, or by accepting an empty store, and then writes the verifier.

This keeps older stores compatible while preventing a wrong passphrase from setting `sealed=false` and failing later during `secret get`.

## Remembered passphrases

- `store init` can persist generated passphrases in the local registry.
- `store seal` removes remembered keys unless `--keep-keys` is used.
- `store unseal` first tries remembered material, then explicit input/prompt.

## Storage paths

See [Architecture](architecture.md) for path-level details (`stores.json`, `secrets/*.vlx`, salts).
