# vaultline

A gittable and automation friendly secret vault with multi store support, a local-only daemon and a convenient auto-completing CLI.

## Quick start

```bash
vaultline daemon --store-dir ./store
vaultline store init mystore ./stores/mystore
vaultline secret set mystore:token --stdin
vaultline secret get mystore:token --out ./token.txt
eval "$(vaultline --output eval-export secret get mystore:token MYSTORE_TOKEN)"
```

## Documentation map

- Project docs index: `docs/index.md`
- CLI reference: `docs/cli.md`
- Daemon behavior + API: `docs/daemon.md`
- Store model and layout: `docs/store-model.md`, `docs/architecture.md`
- Bitwarden import: `docs/bitwarden.md`
- Developer ramp-up: `docs/developer.md`
- Release/version workflow: `docs/runbook.md`

## Development

- Bootstrap local tooling: `./scripts/bootstrap-debian.sh`
- Unit/integration checks:
  - `go test ./...`
  - `smokey --tests-dir tests.d` (Smokey: <https://github.com/micwin/smokey>)

For onboarding details, see `docs/developer.md`.

## Docs/site build

- Install pinned docs toolchain: `python -m pip install -r docs/requirements.txt`
- Build static site (Jekyll + MkDocs): `./scripts/build-site.sh`
- Serve locally: `./scripts/serve-site.sh`

For full docs/release workflow, see `docs/runbook.md` and `RELEASING.md`.
