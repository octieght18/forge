# ADR 0001 — Compose established infrastructure

Date: 5 October 2026  
Status: Accepted requirement from the supplied plan and approved F01.

## Context

Forge has five project hours/week and an API/durable-workflow first release. The project must demonstrate useful integration, developer experience and measured tradeoffs.

## Decision

Build Forge's workload/version API, owner policy, research integration, evidence contract and failure tests. Use established infrastructure for scheduling, durable execution, storage, telemetry, policy and deployment. Do not recreate Backstage, Kubernetes, Temporal, a general-purpose broker or a model-serving engine. A polished chat UI or generic dashboard is outside scope; API examples are the initial developer interface.

This is a build boundary, not a decision to install every named product now. The architecture responsibility table identifies first-release dependencies and later boundaries.

## Alternatives and consequences

- A custom durable job engine hides infrastructure integration and adds recovery semantics we would need to prove. Reject.
- A broad portal/dashboard consumes the available capacity before the execution contract works. Defer.
- Installing the whole later stack now increases setup and operational work without satisfying the reduced release. Defer.

The originality is in composition and evidence. Integration still requires clear ownership and negative tests; an established product does not automatically prove Forge's security or reliability.

## Evidence

[Approved product boundaries](../product-brief.md#9-boundaries-inherited-from-the-supplied-plan), [responsibilities and alternatives](../architecture.md#2-responsibilities-and-release-placement).
