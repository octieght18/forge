# Five-minute technical demo — F57

[D72](../decision-log.md#d72--f57-five-minute-technical-demo) records the boundary.

**Recording status:** A video file was **not** produced in this cloud agent environment (no screen recorder against a live native stack with Keycloak). This document is the timed shot list and narration. Use actual published timings only.

Total target: **5:00**.

| Clock | Shot / action | Narration cue | Evidence / command |
|---|---|---|---|
| 0:00–0:25 | Title: Forge control plane | “Secure self-service registration and durable research without rebuilding Kubernetes, Temporal, or model serving.” | [product-brief.md](../product-brief.md) |
| 0:25–0:55 | Diagram trust boundaries | Personas → Keycloak → API → PostgreSQL; worker/Temporal separate. | [diagrams/architecture.md](../diagrams/architecture.md) |
| 0:55–1:25 | Registration | Create workload + immutable `research.v1` version; tools exactly `corpus.search` / `corpus.read`. | `scripts/api-examples.ps1` or F09 seven-ops JSON; F08 journey median **0.022 s** |
| 1:25–1:55 | Self-service environment | `forge provision` → `202 accepted`; operator apply; **no kubeconfig** to developer. | F19 accept median **4.041 ms**; [self-service-contract.md](../self-service-contract.md) |
| 1:55–2:25 | Tool permissions | Denied document id starts **no** process; secret-shaped errors redacted. | F47 `tool_model_outage` **30695 µs**, 1 evidence row |
| 2:25–2:55 | Async execution | Start run → workflow id `forge-run/<uuid>`; history omits prompts/tokens. | [execution-history.md](../execution-history.md) |
| 2:55–3:25 | Inject failure + inspect | Show F47 table: worker_loss **38118 µs**, 0 tool calls on replay. | `go test -count=1 -timeout 180s -run TestFailureInjection ./internal/failure` |
| 3:25–3:50 | Replay safely | Replay ≠ rerun; F29 replayer **535 µs**, 0 activities. | [replay-modes.md](../replay-modes.md) |
| 3:50–4:20 | Rollback + audit | Canary rollback call **54.421 µs**; audit without token body. | [rollback.md](../rollback.md); F36 |
| 4:20–4:45 | Cost + limits | Free model **$0**; illustrative unapplied EKS floor **~$123/mo**; G3–G6 open. | [capacity-cost-model.md](../capacity-cost-model.md) |
| 4:45–5:00 | Close | Point to quickstart, threat model, simulated incidents. | [operational-evidence-bundle.md](../operational-evidence-bundle.md) |

## Limitations spoken on camera

- No live worker process was killed; Temporal server was not started for F47.
- terraform apply / DNS / TLS / Argo live reconcile were not run.
- Demo uses fixture model unless an operator-held free-route key is present.
