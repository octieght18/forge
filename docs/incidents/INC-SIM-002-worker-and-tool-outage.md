# INC-SIM-002 — Worker resume and tool/model outage (simulated)

**Label:** simulated incident report  
**Date of narrative:** 10 November 2026  
**Evidence basis:** [F47 `worker_loss`](../validation/f47-failure-injection.json), [F47 `tool_model_outage`](../validation/f47-failure-injection.json), [F29 recovery](../f29-validation.md), [execution-stuck runbook](../runbooks/execution-stuck.md)

## Timeline

| Step | Observation |
|---|---|
| T0 | Research run has started retrieval; worker continuity is interrupted (modeled as replay of constructed history, not a process kill). |
| T1 | Resume via replay: **38118 µs**, side effects **0**, tool not called again (`worker_loss`). |
| T2 | Separately, a denied document id and fixture tool/model errors are injected (`tool_model_outage`). |
| T3 | Denied path starts no tool. Tool error redacted. One evidence row and one report stored. Duration **30695 µs**. Duplicate side effects **0**. |
| T4 | Related F29 dependency-failure sample: wall **43361 µs**, `Retrieve` ×2, compensate once, evidence retention `compensated`. |

## Cause

Worker continuity gap plus transient tool/model fixture failures. Retryable classes are `tool_unavailable` and failed configured-model call (3 attempts). `tool_denied` is not retryable and must not start a process.

## Recovery

1. Replay existing Temporal history; do not treat replay as a new model/MCP call ([replay modes](../replay-modes.md)).
2. Inspect history/result without prompts or corpus text.
3. If compensation cannot finish, record `compensation_incomplete`.
4. Follow [execution-stuck](../runbooks/execution-stuck.md) for cancel and uncertain-result paths (F29 cancel **1816 µs**, uncertain **12993 µs**).

## Follow-up

- Keep publishing the honesty limit: no `forge-worker` kill and no Temporal server in F47/F29 samples.
- Retain redaction of secret-shaped tool errors in evidence.
- Corrective action tied to F47: duplicate side-effect count must stay zero on replay after worker_loss.
