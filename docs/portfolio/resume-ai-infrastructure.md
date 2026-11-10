# Resume variant — AI Infrastructure / Agent Platform

Evidence-backed bullets emphasizing durable execution and tenant security. No invented employer history.

## Summary

AI infrastructure engineer focused on durable agent workflows, tool permission boundaries, and tenant-aware control planes: Temporal-backed research execution over a fixed corpus with safe replay, compensation, and measured recovery.

## Selected evidence bullets

- Implemented Temporal research workflow with workflow id `forge-run/<uuid>`, 2m workflow timeout, 30s activity start-to-close, 10s heartbeats, and classified retries (`tool_unavailable` / model failures retryable ×3; `tool_denied` not retryable).
- Bounded tools to `corpus.search` and `corpus.read` with allow-list checks before MCP process start; fixture path avoids live model keys in tests ([mcp-adapter.md](../mcp-adapter.md)).
- Selected free OpenRouter model `google/gemma-4-26b-a4b-it:free` with **no paid fallback**; token price **$0** on catalog date; evidence citations validated in fixtures ([model-execution.md](../model-execution.md)).
- Measured durable recovery: dependency failure **43361 µs** (Retrieve×2, compensate×1); cancel **1816 µs**; replayer **535 µs** with **0** activity entries ([f29-validation.md](../f29-validation.md)).
- Failure injection: worker-loss replay **38118 µs** / **0** side effects; tool/model outage **30695 µs** with redacted secrets; quota exhaustion **16572 µs** at cap one ([f47-validation.md](../f47-validation.md)).
- Enforced tenant/project scoping (other-tenant **404**), service-token **403** on product routes, owner/operator matrix, and per-process rate/quota with **429 Retry-After: 1** ([threat-model.md](../threat-model.md)).
- In-process audit/metering without storing prompts or tokens; retention rules refuse demo cleanup of retained DB/secrets ([f36-validation.md](../f36-validation.md), [retention.md](../retention.md)).

## Honest limits

G3/G4 open. No live worker kill or Temporal server in F47/F29 samples. Not organization multi-tenant. Not a production security certification.
