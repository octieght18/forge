# Architecture decision records

F02 records accepted architecture boundaries on 5 October 2026. Later ADRs and tickets refine runtime behavior. Diagrams: [diagrams/architecture.md](../diagrams/architecture.md).

| Record | Decision / status | Alternatives (summary) | Measured tradeoffs / evidence |
|---|---|---|---|
| [0001 — Compose existing infrastructure](0001-compose-existing-infrastructure.md) | Accepted build boundary inherited from the supplied plan/F01 | Rebuild K8s/Temporal/model serving in-house — rejected | Composition kept; F41 EKS module exists but **not applied**; no cloud spend ([f41-validation.md](../f41-validation.md)) |
| [0002 — API, state and durable execution](0002-api-state-and-durable-execution.md) | Accepted Go HTTP/OpenAPI, PostgreSQL, Temporal and crash-safe handoff | Public gRPC; Temporal-only product storage; custom broker — rejected initially | F10 uncapped 1196–2148 req/s at 1–2 clients; product cap 200 req/s; F12 reconciliation; F29 recovery µs samples ([f10-validation.md](../f10-validation.md), [f29-validation.md](../f29-validation.md)) |
| [0003 — Local trust and research](0003-local-trust-and-research.md) | Accepted Keycloak OIDC, action matrix, bounded MCP/evidence | Open-ended agent tools; header-based identity — rejected | F33 service token 403; F34 RBAC; F40 security suite on disposable DB; G3/G4 still open ([threat-model.md](../threat-model.md)) |
| [0004 — Model and deployment target](0004-model-and-cloud.md) | Accepted OpenRouter/free model; cloud deferred then planned as EKS docs only | Paid models; dual cloud targets — rejected for demo | Free token price $0 (catalog 5 Oct 2026); F49 illustrative EKS floor ~$122.63/mo **unapplied** ([capacity-cost-model.md](../capacity-cost-model.md)) |
| [0005 — Native local deployment](0005-native-local-deployment.md) | Owner accepted native WSL services; later D26 allows kind | Compose-first only — superseded | F08 startup median 66.5 s; occupied-port recovery 64.7 s after fixture release ([f08-baseline.md](../f08-baseline.md)) |
| [0006 — Separate operator environment controller](0006-workload-environment-controller.md) | Restored CRD/controller; read-only DB role; no kubeconfig to developers | In-API privileged apply; developer kubeconfig — rejected | F18 recovery 9.4 ms fake-client; F19 three template provisions; G2 recorded ([g2-validation.md](../g2-validation.md)) |

## Cross-cutting choices linked to measurements

| Choice | Why | Evidence |
|---|---|---|
| Temporal history is execution SoT; PostgreSQL holds runs/commands | Avoid dual status machines | F27 history/result; F47 worker_loss replay 0 tool calls |
| Workflow id `forge-run/<uuid>`; reuse rejected | Stable dispatch identity | execution-timeouts / F26 |
| Retryable: `tool_unavailable`, failed model call (3×); not: `tool_denied`, `model_invalid`, `identity_mismatch` | Bound side effects | [retry-compensation.md](../retry-compensation.md), F25/F29 |
| Canary pin promote local→staging; cloud refused | No paid environment created | F44/F45; rollback 54 µs sample |
| Per-process rate/quota; audit in-process | Demo scope | F35/F36; F47 quota_exhaustion |

[Architecture and comparison tables](../architecture.md) contain source links, costs, reversibility and future test obligations. [Owner decision log](../decision-log.md) records consultation responses. [F02 validation](../f02-validation.md) distinguishes documentation checks from unimplemented runtime behavior. [F52 verification](../f52-validation.md) covers this diagram/ADR packaging.
