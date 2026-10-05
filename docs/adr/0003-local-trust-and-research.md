# ADR 0003 — Local identity and bounded research integration

Date: 5 October 2026  
Status: Accepted by the owner on 5 October 2026; Keycloak OIDC selected.

## Decision

Use local Keycloak OIDC identity and central Go authorization from the first runtime, as selected by the owner. Persist identity data in a separate database/role. Developers register/version/run/read/cancel/rerun their own workloads/runs; the operator can inspect all but cannot mutate another owner's records. Use authorization code with PKCE for login and validate API access tokens at the Go API. A reviewed deployment trust model is still required before remote multi-user exposure. No new OPA service is needed for these fixed rules.

Use one worker-owned read-only MCP process over stdio, one immutable public/synthetic corpus shared by initial owners, and fixed bounded retrieve/generate/validate stages. Evidence records and reports stay owner-private. Only the selected model backend receives bounded retrieved excerpts and the question. It cannot invoke arbitrary tools or grant permissions.

## Alternatives and consequences

- Static local tokens: simpler local setup but the owner chose OIDC; reject. Dex/existing provider are alternatives; Keycloak was selected for the reproducible local identity setup.
- Operator cross-owner cancellation: useful incident control but exceeds approved inspection; requires explicit additional authorization policy.
- Streamable HTTP MCP: supports remote separation, adds transport/exposure controls; defer until needed.
- Open-ended agent loop: more flexible, adds unbounded tool/cost/control behavior; defer.
- Per-owner corpus storage: useful later but expands ingestion/isolation scope; initial corpus is common fixtures.
- OPA versus Kyverno: general decisions versus cluster resource policy; neither replaces the API's enforcement point. Later dynamic application policy can use OPA; later Kubernetes policy can use Kyverno after its own decision.

The local system is not a hostile-code sandbox. Tokens, database credentials and provider credentials remain outside Git and telemetry. Citation integrity is machine checked; claim support needs scenario evaluation and review. Retention/budget/performance values are not selected here.

## Evidence and future checks

[Action matrix and trust envelope](../architecture.md#6-security-and-deployment-envelope), [research/evidence semantics](../architecture.md#5-research-evidence-and-model-boundary), [MCP transports](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports).
