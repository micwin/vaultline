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
# Startet den lokalen Daemon (Default: 127.0.0.1:8428)
vaultline daemon --store-dir ./store

# Legt einen zusätzlichen Store an und registriert ihn
vaultline store init ai ./stores/ai

# Speichert ein Secret interaktiv (Eingabe wird nicht angezeigt)
vaultline secret set ai:token --stdin

# Liest ein Secret in eine Datei
vaultline secret get ai:token --out ./token.txt

# Cross-store-Unseal: Passphrase aus anderem Store lesen
vaultline store unseal ai --from-secret superstore:ai.unseal
```

## Recent operations focus

- `secret copy` / `secret move` with collision-safe default and explicit `--force`
- `store unseal --from-secret <store:key>` to retrieve unseal material from another store
- reproducible smokey suite with fully isolated runtime state

## Docs and releases

- use [Downloads]({{ '/downloads/' | relative_url }}) for release artifacts
- use `RELEASING.md` in repo root for release flow
