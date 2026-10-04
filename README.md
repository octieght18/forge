# Forge

A proposed self-service control plane for asynchronous AI workloads.

Forge's purpose is to show how an engineering team can compose existing infrastructure into a platform that developers consume: register a workload, request a controlled execution environment, run a durable workflow, and inspect the result and its evidence.

## Current work

**F01 — Write the internal platform product brief** is ready for review. This repository contains the brief and the owner's product decisions, not an implemented platform.

The accepted first release is the **API and durable research workflow only**: one engineering team, owner-private workloads/runs, operator visibility, and a fixed local document corpus accessed through read-only MCP tools. Available project capacity is five hours/week. Environment provisioning and broader platform capabilities follow in later releases.

- [Internal product brief](docs/product-brief.md)
- [Open product and design decisions](docs/decision-log.md)

The owner must review product and design choices before they become accepted decisions. Architecture selection belongs to F02; no framework, cloud, model provider, or deployment approach is selected by this draft.

## Delivery plan

1. Product requirements and Go API foundation.
2. Kubernetes self-service provisioning.
3. Durable AI workflows and safe debugging.
4. Tenant security, policy, quotas, audit, and metering.
5. GitOps deployment and operations.
6. Measured evidence, documentation, and portfolio.

The Wekan project contains the original full execution backlog. Its dates and phase assignments need revision to reflect the accepted smaller first release and five project hours/week. Read the brief for the current scope.
