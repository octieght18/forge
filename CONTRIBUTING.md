# Contributing to Forge

Forge is a portfolio control-plane project. Product decisions are recorded in [docs/decision-log.md](docs/decision-log.md). Prefer small, tested changes that keep documented limits honest.

## Before you start

1. Read [docs/quickstart.md](docs/quickstart.md) and [docs/product-brief.md](docs/product-brief.md).
2. Do not mark gates G3–G6 done unless the owner explicitly accepts them.
3. Do not commit secrets, `.env` files, kubeconfigs, access tokens, Terraform state, `backend.hcl`, FerretDB `state.json`, or `.cursor/` paths.
4. Do not invent live cloud spend, signed images, or measurements you did not take.

## Development loop

```sh
go test ./...
python3 scripts/check-docs.py
```

On Windows with PowerShell available, also run the documented PowerShell checker when you change scripts. Linux cloud VMs may lack that checker; record the gap.

Use disposable PostgreSQL for integration tests. Do not point harnesses at the retained product database.

## Pull requests

- Keep scope to one ticket or one coherent fix.
- Update `docs/decision-log.md` and a `docs/fNN-validation.md` when behavior or claims change.
- Link evidence (test commands, JSON samples). Prefer failing honestly over unverifiable claims.
- Repository visibility changes (making GitHub public) are an **owner-only** explicit action. Contributors must not change visibility.
