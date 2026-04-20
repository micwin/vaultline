---
layout: default
title: Overview
---

# vaultline

`vaultline` is a local-first secret vault with a clear operational model:

- sealed by default
- explicit unseal flows
- deterministic secret file layout
- automation-friendly CLI + daemon API

## Quick start

```bash
# Assumes the local daemon is already running on default address (127.0.0.1:8428)

# Create and register a named store
vaultline store init mystore ./stores/mystore

# Save a secret interactively (input stays hidden)
vaultline secret set mystore:token --stdin

# Read a secret into a file
vaultline secret get mystore:token --out ./token.txt

# Cross-store unseal: read unseal material from another store
vaultline store unseal mystore --from-secret superstore:mystore.unseal
```

## Recent operations focus

- `secret copy` / `secret move` with collision-safe default and explicit `--force`
- `store unseal --from-secret <store:key>` to retrieve unseal material from another store
- reproducible smokey suite with fully isolated runtime state

## Docs and releases

- use [Downloads]({{ '/downloads/' | relative_url }}) for release artifacts
- use `RELEASING.md` in repo root for release flow
