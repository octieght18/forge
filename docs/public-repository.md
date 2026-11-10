# Public-ready repository hygiene — F51

[D66](decision-log.md#d66--f51-public-repository-hygiene) records the boundary.

## Gate status

**G5 Operational deployment and recovery remains Backlog / open.** This ticket organizes publishable artifacts and does **not** close G5. Making the GitHub repository public is a separate owner action and was **not** performed by this change.

## Layout

| Area | Location |
|---|---|
| Source | `cmd/`, `internal/` |
| Examples | `scripts/*-examples.ps1`, `docs/api-examples.md`, templates via `forge template` |
| Docs | `docs/` (quickstart, ADRs, validation, runbooks, incidents, deployment) |
| CI | `.github/workflows/ci.yml` (format, vet, test, docs, policy, supply-chain checks) |
| Deploy manifests | `deploy/` (kubernetes, gateway, gitops, terraform/eks — cloud not applied) |
| License / contributing | [LICENSE](../LICENSE), [CONTRIBUTING.md](../CONTRIBUTING.md), [SECURITY.md](../SECURITY.md) |
| Env template | [.env.example](../.env.example) (no secrets) |
| Release checklist | [release-checklist.md](release-checklist.md) |

## Secrets and personal data scan (10 November 2026)

Scanned the tree for private keys, `.env`, AWS key patterns, JWT-shaped literals in non-test paths, and FerretDB `state.json`. Findings:

| Finding | Disposition |
|---|---|
| Test fixtures using strings like `Bearer` / `password` / `secret` | Retained; redaction tests assert they do not leak |
| Operator docs that already redact token material | Retained |
| `.gitignore` covers `.env`, `*.pem`, `*.key`, Terraform state, `backend.hcl`, `docs/profiles/` | Extended for `.cursor/` and common token filenames |
| No committed `.env`, kubeconfig, or cosign private key | Confirmed in this scan |
| Owner first name `ahmad` in decision log / product docs | Retained as project owner attribution, not a secret |

## Publication rule

Do not flip GitHub visibility from this repository's automation. The owner publishes deliberately after the [release checklist](release-checklist.md).
