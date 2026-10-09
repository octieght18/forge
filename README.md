# Forge

A proposed self-service control plane for asynchronous AI workloads.

Forge's purpose is to show how an engineering team can compose existing infrastructure into a platform that developers consume: register a workload, request a controlled execution environment, run a durable workflow, and inspect the result and its evidence.

## Current work

Start with the [local quickstart](docs/quickstart.md), [current API examples](docs/api-examples.md) and [repository map](docs/repository-map.md). These cover native startup, real login, loading a Bearer token into a client and all seven available product operations.

**F01/F02** establish the product and architecture; **F03** supplies the HTTP foundation; **F04/F05** add the contract and PostgreSQL persistence. **F06** implements authenticated workload/version registration, owner checks, ETag updates, signed cursor pages and corpus approvals. **F07** deploys PostgreSQL, Keycloak and the API natively in Ubuntu WSL, with private retained data and real browser/PKCE login. **F08** captures repeatable startup, registration and occupied-port recovery observations with raw evidence and an unmeasured human worksheet. **F09** publishes the first native quickstart, API examples and repository navigation with clean-checkout verification. **F10** validates cancellation/rollback/draining across dependencies, corrects disconnect handling during JWKS waits, and publishes reduced registration CPU/heap profiles and latency/error observations. Run HTTP endpoints and Temporal delivery follow later tickets.

The accepted first release is the **API and durable research workflow only**: one engineering team, owner-private workloads/runs, operator visibility, and a fixed local document corpus accessed through read-only MCP tools. Available project capacity is five hours/week. Environment provisioning and broader platform capabilities follow in later releases.

**F11** defines the fixed research specification and submission/cancellation/execution lifecycle, with executable response consistency checks and ambiguity/race examples. Its original Kubernetes environment checklist is deferred by owner-approved scope amendment; run endpoints and dispatch remain later work.

**F12** supplies PostgreSQL-backed command reconciliation with lease fencing, persisted bounded retries, start-before-cancel and a separate dispatcher role. Native startup prepares its storage/credentials; the Temporal adapter and dispatcher service remain F22 work.

The owner approved [containerized Kubernetes deployment in WSL](docs/kubernetes-local.md): API, PostgreSQL and Keycloak in kind, with copied existing databases and retained native originals. Public loopback URLs and owner identities stay the same. **F12a/F13** restore the environment CRD/controller and provision [one resource/runtime boundary per workload](docs/execution-environments.md), with replay, drift repair and protected cleanup. Research workers and enforced network isolation remain future work.

**F14** adds a [developer CLI](docs/developer-cli.md) over the existing registration API. `register` creates a workload, `deploy` records an immutable version, `status` reads those records, and `delete` reports the API's rejection of deletion. It does not provision an environment or start a run.

**F15** adds [service and MCP agent templates](docs/workload-templates.md) through `forge template`. Both use the fixed research contract. Rendering writes local files only.

- [Internal product brief](docs/product-brief.md)
- [Product and design decisions](docs/decision-log.md)
- [Architecture and integration boundaries](docs/architecture.md)
- [Architecture decision records](docs/adr/README.md)
- [F02 documentation validation](docs/f02-validation.md)
- [Go service setup and conventions](docs/service-foundation.md)
- [F03 validation](docs/f03-validation.md)
- [API contract and conventions](docs/api-contract.md)
- [OpenAPI 3.1.1](internal/contract/openapi.json)
- [F04 validation](docs/f04-validation.md)
- [PostgreSQL persistence and migration guide](docs/persistence.md)
- [F05 validation](docs/f05-validation.md)
- [Authenticated registration setup and behavior](docs/registration-api.md)
- [F06 validation](docs/f06-validation.md)
- [Native local startup, login and retained data](docs/local-stack.md)
- [F07 validation](docs/f07-validation.md)
- [Baseline procedure and measurement commands](docs/baseline-procedure.md)
- [F08 observed timings and raw evidence](docs/f08-baseline.md)
- [F09 quickstart validation](docs/f09-validation.md)
- [F10 cancellation validation and profiling results](docs/f10-validation.md)
- [Reproducible registration profiling procedure](docs/profile-procedure.md)
- [Accepted Balanced local performance targets](docs/performance-targets.md)
- [Research specification and lifecycle](docs/workload-lifecycle.md)
- [F11 verification](docs/f11-validation.md)
- [Durable command reconciliation and upgrade](docs/command-reconciliation.md)
- [F12 verification](docs/f12-validation.md)
- [Kubernetes migration and local commands](docs/kubernetes-local.md)
- [Kubernetes verification](docs/kubernetes-validation.md)
- [Operator environment provisioning](docs/execution-environments.md)
- [F12a/F13 verification](docs/f13-validation.md)
- [Developer CLI](docs/developer-cli.md)
- [F14 verification](docs/f14-validation.md)
- [Workload templates](docs/workload-templates.md)
- [F15 verification](docs/f15-validation.md)

The owner must review product and design choices before they become accepted decisions. F02 records the accepted Go HTTP/OpenAPI, PostgreSQL and Temporal foundation, OpenRouter backend and local Kubernetes target. Keycloak OIDC is included from the first release. The testing model is `google/gemma-4-26b-a4b-it:free`. D26 supersedes F07's no-container deployment choice: the current stack can now run in kind inside WSL. F03's HTTP defaults and lifecycle conventions are accepted; new API/identity/workflow decisions still require consultation.

## Run the foundation

With Go 1.27.1 installed:

```sh
go run ./cmd/api
```

This default command serves process health only. For authenticated registration with the native database and issuer, run `scripts/local-stack.ps1 up` after the documented prerequisites. Read the [native stack guide](docs/local-stack.md) for startup/login/data locations and the [service guide](docs/service-foundation.md) for configuration and tests.

## Delivery plan

1. Product requirements and Go API foundation.
2. Kubernetes self-service provisioning.
3. Durable AI workflows and safe debugging.
4. Tenant security, policy, quotas, audit, and metering.
5. GitOps deployment and operations.
6. Measured evidence, documentation, and portfolio.

The Wekan project contains the original full execution backlog. Its dates and phase assignments need revision to reflect the accepted smaller first release and five project hours/week. Read the brief for the current scope.
