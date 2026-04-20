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
go run ./cmd/vaultline daemon --addr 127.0.0.1:8428 --store-dir ./store
go run ./cmd/vaultline --addr 127.0.0.1:8428 store init ai ./stores/ai
go run ./cmd/vaultline --addr 127.0.0.1:8428 secret set ai:token --stdin
go run ./cmd/vaultline --addr 127.0.0.1:8428 secret get ai:token --out ./token.txt
```

## Recent operations focus

- `secret copy` / `secret move` with collision-safe default and explicit `--force`
- `store unseal --from-secret <store:key>` to retrieve unseal material from another store
- reproducible smokey suite with fully isolated runtime state

## Docs and releases

- use [Downloads]({{ '/downloads/' | relative_url }}) for release artifacts
- use `RELEASING.md` in repo root for release flow

