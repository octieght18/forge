# ADR 0002 — HTTP API, product state and durable execution

Date: 5 October 2026  
Status: Accepted by the owner on 5 October 2026.

## Context and decision

Choose a Go HTTP/JSON API with OpenAPI, PostgreSQL for product records, and Temporal for durable execution. Start infrastructure locally with Docker Compose; Kubernetes deployment/provisioning follows later. Product and workflow persistence have separate databases and roles even if one local PostgreSQL server hosts them.

PostgreSQL commits the run request and pending start command atomically. An idempotent dispatcher starts the stable Temporal Workflow ID and recovers ambiguous delivery by lookup. Cancellation uses the same ordered durable handoff. This small product-to-Temporal bridge is not a replacement for Temporal's task queues. Temporal owns execution status/history; PostgreSQL holds identity, ownership and evidence plus explicitly staleable projections.

Use bounded activities for external I/O, idempotent artifact writes and explicit replay/rerun distinctions. Do not promise exactly-once model calls. No Redis or independent broker is selected.

## Alternatives and tradeoffs

- Public gRPC: useful generated clients/streaming, extra contract/tooling; no demonstrated first-release requirement.
- Both public transports: doubles policy/contract coverage; reject initially.
- Direct database insert then unrecorded Temporal start: loses the handoff on a crash; reject.
- Temporal-only product storage: couples ownership/record queries to workflow retention and engine data; reject.
- Custom job scheduler, Redis or broker: duplicates existing execution responsibility; revisit with evidence.
- Kubernetes first: closer to the later platform, but adds local resource/setup work at the reduced capacity.

HTTP routing and model providers are reversible behind boundaries. Replacing Temporal is more expensive: execution history and replay are engine-specific. PostgreSQL migrations and stored records require deliberate migration, not a config switch.

## Verification

The architecture's [crash-safe handoff](../architecture.md#4-state-ownership-and-crash-safe-submission) and [test matrix](../architecture.md#9-verification-obligations-for-implementation) define future behavioral checks. No implementation has passed them yet.
