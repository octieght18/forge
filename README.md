# Forge

A proposed self-service control plane for asynchronous AI workloads.

Forge's purpose is to show how an engineering team can compose existing infrastructure into a platform that developers consume: register a workload, request a controlled execution environment, run a durable workflow, and inspect the result and its evidence.

## Current work

**F01** and **F02** establish the accepted product and architecture. **F03** adds the runnable Go HTTP service foundation. **F04** supplies the accepted API contract and validation. **F05 — Persist control-plane state in PostgreSQL** adds migrations and repositories for workloads, immutable versions, runs and durable start/cancel intents. Product HTTP endpoints and OIDC enforcement follow their implementation tickets.

The accepted first release is the **API and durable research workflow only**: one engineering team, owner-private workloads/runs, operator visibility, and a fixed local document corpus accessed through read-only MCP tools. Available project capacity is five hours/week. Environment provisioning and broader platform capabilities follow in later releases.

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

The owner must review product and design choices before they become accepted decisions. F02 records the accepted Go HTTP/OpenAPI, PostgreSQL and Temporal foundation, OpenRouter backend and local Kubernetes target. Keycloak OIDC is included from the first release. The testing model is `google/gemma-4-26b-a4b-it:free`; Docker Compose comes first and kind later. F03's HTTP defaults and lifecycle conventions are accepted; new API/identity/workflow decisions still require consultation.

## Run the foundation

With Go 1.27.1 installed:

```sh
go run ./cmd/api
```

Open `http://127.0.0.1:8081/healthz` or `/readyz`. Only process health is implemented so far. Read the [service guide](docs/service-foundation.md) for configuration, tests and this PC's portable Go setup.

## Delivery plan

1. Product requirements and Go API foundation.
2. Kubernetes self-service provisioning.
3. Durable AI workflows and safe debugging.
4. Tenant security, policy, quotas, audit, and metering.
5. GitOps deployment and operations.
6. Measured evidence, documentation, and portfolio.

The Wekan project contains the original full execution backlog. Its dates and phase assignments need revision to reflect the accepted smaller first release and five project hours/week. Read the brief for the current scope.
