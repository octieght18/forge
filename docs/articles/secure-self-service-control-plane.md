# Secure self-service control plane for asynchronous AI workloads

**Without rebuilding Kubernetes, Temporal, or model serving**

[D73](../decision-log.md#d73--f58-technical-architecture-article) · November 2026 · Forge portfolio article

## Problem

Engineering teams that ship AI agents repeatedly re-integrate identity, registration, environment isolation, durable execution, and debugging. Forge is a small control plane that composes existing infrastructure so developers register a workload, request an environment, run a durable research workflow, and inspect evidence—while operators keep policy and recovery levers.

## Architecture

Forge exposes a Go HTTP/OpenAPI API. PostgreSQL stores workloads, immutable versions, runs, and start/cancel commands. Temporal owns execution history; workflow ids are `forge-run/<run UUID>` and reuse is rejected. A Go worker runs bounded activities against exactly two corpus tools (`corpus.search`, `corpus.read`) and the free OpenRouter model `google/gemma-4-26b-a4b-it:free` (no paid fallback). An environment controller applies per-workload Kubernetes boundaries with a read-only DB role; developers never receive a kubeconfig.

Diagrams: [diagrams/architecture.md](../diagrams/architecture.md). ADRs: [adr/README.md](../adr/README.md).

## Tenancy and trust

One engineering team with multiple owners (D02/D05). Owner-private mutation; operator cross-owner read; other-tenant ids return 404; service tokens get 403 on product routes. Threats, controls, tests, and gaps are tabulated in [threat-model.md](../threat-model.md). **G3 and G4 remain open**—demo controls are not production isolation.

## Durability and replay

Crash-safe submission commits the run and start command before `202`. Retryable failures (`tool_unavailable`, failed configured-model call) retry up to three times; `tool_denied`, `model_invalid`, and `identity_mismatch` do not. Terminal states distinguish compensation completed vs incomplete. Replay reads history and must not re-call tools/models; re-execution creates a new run ([replay-modes.md](../replay-modes.md)).

## Actual results (measured)

| Area | Result |
|---|---|
| Native registration journey | F08 median **0.022 s** (10 samples) |
| Uncapped registration profile | F10 **1196–2148 req/s** at 1–2 clients; product cap **200 req/s** |
| Template provision accept | F19 median **4.041 ms** (n=3 kinds) |
| Worker-loss replay | F47 **38118 µs**, **0** tool calls |
| Backup dump / restore check | F46 **61.76 ms** / **71.79 ms** |
| Canary rollback call | F45 **54.421 µs** |
| Illustrative unapplied EKS floor | F49 **~$122.63/mo** (public list, 2026-10-10) |
| Model token price | **$0** on the selected free route |

## Tradeoffs

- Compose, don't rebuild: faster integration, but Kind NetworkPolicy is unenforced and cloud apply was never run.
- In-process audit/meters and per-process rate limits simplify the demo and lose durability/distribution.
- Free model removes token cost and forbids silent paid fallback when the route disappears.
- Honesty over marketing: simulated incidents, unsigned images, and open gates are documented rather than papered over.

## Reproducible artifacts

Quickstart ([quickstart.md](../quickstart.md)), OpenAPI (`internal/contract/openapi.json`), failure suite ([failure-injection.md](../failure-injection.md)), demo script ([demo/five-minute-demo.md](../demo/five-minute-demo.md)), cost/ops bundle ([operational-evidence-bundle.md](../operational-evidence-bundle.md)).

No unverifiable scale, security certification, or adoption claim is made here.
