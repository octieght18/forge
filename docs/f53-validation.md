# F53 verification — 10 November 2026

F53 validates the published quickstart and API documentation as far as this environment allows. [D68](decision-log.md#d68--f53-quickstart-and-api-docs-validation) records the boundary.

## Trial that ran (cloud Linux VM)

| Step | Result |
|---|---|
| Native WSL / PowerShell quickstart (`scripts/local-stack.ps1`) | **Not run** — no Windows host, no `pwsh`, no native PostgreSQL/Keycloak on this VM |
| kind / Kubernetes stack | **Not run** — `kind`/`kubectl` absent |
| Real Keycloak PKCE login | **Not run** |
| `scripts/api-examples.ps1` live seven operations | **Not run** (PowerShell absent). Script present (3548 bytes) and still referenced by docs |
| OpenAPI load + path inventory | **Pass** — 13 paths; `/api/v1/workloads` present (`internal/contract/openapi.json`) |
| `go test ./internal/contract ./internal/cli ./internal/template ./internal/httpapi ./internal/execution ./internal/provision` | **Pass** (~1–2 s wall for the contract/cli/template bundle) |
| Docs link/fence check | **Pass** — `python3 scripts/check-docs.py` |
| Stale doc claims | **Fixed** — quickstart/api-examples no longer say runs/provision/quotas are unimplemented |

Prior clean-checkout evidence with real Keycloak remains [validation/f09-local-quickstart.json](validation/f09-local-quickstart.json) (7 Oct 2026): seven registration operations verified against the owner's native stack.

## Capability checks via existing automated tests

| Area | Evidence in this session |
|---|---|
| Registration | `internal/contract`, `internal/cli`, `internal/httpapi` packages ok |
| Provisioning acceptance | `internal/provision`, template/CLI packages ok; F19 numbers reused historically |
| Execution / cancel / recovery | `internal/execution` ok (SDK fixtures; no Temporal server) |
| Quota errors | Covered in httpapi package tests when DB URL set; this VM had no `pg_isready` / admin DB URL, so disposable-DB cases follow CI/`FORGE_TEST_ADMIN_DATABASE_URL` |

## Onboarding time and manual steps

| Measure | Value |
|---|---|
| This session's automated doc+contract trial wall time | about **1–2 seconds** of Go tests after module cache warm; docs check sub-second |
| Human native onboarding (install WSL, packages, first Keycloak login, typing) | **Still unmeasured** — [baseline-human-worksheet.json](baseline-human-worksheet.json) remains null |
| Manual steps that remain after token exists | Login refresh every five minutes; operator `environment.ps1 apply`; no developer kubeconfig; cloud apply refused |

## Limits

G5 stays open. This F53 trial is **not** a clean Windows/WSL installation. PowerShell documentation checker was unavailable. No production SLO.
