# Runbook — execution stuck

## Symptoms

- A run id was returned, but history or result never reaches a terminal state (`complete`, `fail_closed`, `canceled`, `timed_out`).
- The worker appears silent; tool or model calls do not progress.
- Cancel was submitted and the run still looks active.

## Immediate checks

1. Confirm workflow id shape `forge-run/<run UUID>`. Reuse of an existing workflow id is rejected ([execution timeouts](../execution-timeouts.md)).
2. Read authenticated history and result. They omit prompts, corpus text, and tokens ([execution history](../execution-history.md)).
3. Confirm Temporal history is the execution source of truth. PostgreSQL holds the run and start/cancel commands only.
4. Classify the last failure: retryable (`tool_unavailable`, failed configured-model call; max 3 attempts) versus not retryable (`tool_denied`, `model_invalid`, `identity_mismatch`).

## Recovery

1. **Worker loss / resume:** prefer replay of existing history. [F47 worker_loss](../failure-injection.md) measured 38118 µs with zero tool calls and zero duplicate side effects on constructed history. [F29](../f29-validation.md) replayer measured 535 µs with zero activity entries. Do not start a new run unless re-execution is intentional ([replay modes](../replay-modes.md)).
2. **Tool or model outage:** wait for bounded retries (activity start-to-close 30s, heartbeat 10s, workflow timeout 2m). [F47 tool_model_outage](../failure-injection.md) measured 30695 µs, one evidence row, secret-shaped tool error redacted. [F29](../f29-validation.md) dependency failure measured 43361 µs wall time with `Retrieve` ×2 and compensation once.
3. **Cancel:** submit cancel through the API. Cancellation during retrieval in F29 reached terminal `canceled` in 1816 µs (SDK test clock). If compensation does not finish, the record must say `compensation_incomplete`.
4. **Uncertain result:** an insufficient-evidence terminal is valid; F29 measured 12993 µs with zero evidence rows. Do not invent citations.

## Corrective actions from F47

- Treat worker restart as history replay until a live Temporal server kill is demonstrated. No `forge-worker` process was killed in F47 or F29.
- Keep evidence retention `compensated` when retrieval committed without a report ([retry compensation](../retry-compensation.md)).

## Limits

These timings are SDK / in-process fixture samples. No Temporal server, OpenRouter call, or MCP process was started in the F47 suite. Workflow timeout and heartbeat numbers are product configuration, not newly measured production SLOs.
