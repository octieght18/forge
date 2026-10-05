# Architecture decision records

F02 records accepted architecture boundaries on 5 October 2026. Detailed runtime implementation follows in later tasks.

| Record | Decision / status |
|---|---|
| [0001 — Compose established infrastructure](0001-compose-existing-infrastructure.md) | Accepted build boundary inherited from the supplied plan/F01 |
| [0002 — API, state and durable execution](0002-api-state-and-durable-execution.md) | Accepted Go HTTP/OpenAPI, PostgreSQL, Temporal and crash-safe handoff |
| [0003 — Local trust and research](0003-local-trust-and-research.md) | Accepted Keycloak OIDC, action matrix, bounded MCP/evidence boundary |
| [0004 — Model and deployment target](0004-model-and-cloud.md) | Accepted OpenRouter, Compose first/kind later; free model selected under delegation |

[Architecture and comparison tables](../architecture.md) contain source links, costs, reversibility and future test obligations. [Owner decision log](../decision-log.md) records consultation responses. [F02 validation](../f02-validation.md) distinguishes documentation checks from unimplemented runtime behavior.
