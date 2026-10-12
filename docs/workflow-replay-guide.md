# Workflow and replay guide — F30

This is the operator and developer guide for the research workflow. It collects the design that F22 through F29 implemented. The numbers below are one SDK sample. They are not a production SLO.

## Where state lives

PostgreSQL stores the run and the start or cancel command. Temporal stores execution history. There is no second execution-status column. The workflow id is `forge-run/<run UUID>`. Reuse of that id is rejected.

The worker is `cmd/forge-worker`. The dispatcher is `cmd/forge-dispatch`. The local Temporal dev server is [scripts/temporal-dev.ps1](../scripts/temporal-dev.ps1). The native stack does not start it. This session did not start it either.

## Retries

`tool_unavailable` and a failed call to a configured model may run again. `tool_denied`, `model_invalid`, and an identity mismatch may not. A missing model configuration finishes as `fail_closed` and does not spend the retry budget. The budget is three attempts, one second then two seconds, capped at four seconds. A successful activity is stored under an idempotency key and is not repeated. Details are in the [retry guide](retry-compensation.md).

## Compensation and uncertainty

Evidence left after retrieval when no report is stored is marked `compensated`. Compensation does not reverse a hosted model charge. Doing it again does not change the row.

`insufficient_evidence` is a completed result, not an activity failure. The F29 sample stored no evidence row for a document the fixture does not contain.

## Evidence

The worker keeps passages in memory with the run, workflow, snapshot, passage, excerpt hash, and retention `workflow_history`. The public history and result routes do not return the excerpt, the question, or a bearer token. Evidence ids on the result are lowercase UUID version 4 values.

## Replay safety

Replay runs workflow code against recorded history. Activity functions are not entered. The model and MCP are not called. Retry is the activity budget above. Re-execution is `POST /api/v1/runs/{run_id}/rerun`: a new run and a new workflow id linked by `rerun_of`. That path can call a tool. Replay cannot.

`FORGE_DEBUG=true` forces the in-process corpus fixture and the fixture model. A configured `FORGE_MCP_COMMAND` or `FORGE_MODEL_API_KEY` is ignored on that path. Do not point a debug run at the live OpenRouter API.

## Operator procedure

1. Leave `FORGE_MODEL_API_KEY` unset. For a debug worker, set `FORGE_DEBUG=true`.
2. Start Temporal only with `scripts/temporal-dev.ps1` when a server is required. The API reads history only when `TEMPORAL_ADDRESS` is set. Without it, history and result return 503 and the run read stays available.
3. Inspect a run with `GET /api/v1/runs/{run_id}/history` and `GET /api/v1/runs/{run_id}/result`. History shows step, attempt, time, and a fixed failure sentence.
4. To reproduce the failure sample without a server, from the repository root:

```text
go test -count=1 -v -run TestDurableRecoveryMeasurements ./internal/execution
```

## Recorded failure timeline

This timeline is the SDK log from the F29 sample, not a server history download.

1. `Retrieve` attempt 1 returned `tool_unavailable` (`dependency down`).
2. The suite skipped the one-second retry timer (`TimeSkipped 1s`). Wall time did not include that sleep.
3. `Retrieve` attempt 2 completed. The tool had been called twice. One evidence row was stored.
4. `Generate` ran once. No model was configured, so the class was `model_unavailable` and the retry budget was not spent.
5. `Compensate` marked the evidence row `compensated`. A second row was not created.
6. `RecordTerminal` stored `fail_closed`.

Wall times from that same run:

| Case | Wall time |
|---|---|
| Dependency failure through compensation | 43361 µs |
| Uncertain `insufficient_evidence` result | 12993 µs |
| Cancellation during retrieval, terminal `canceled` | 1816 µs |
| Replayer over recorded history, zero activity entries | 535 µs |

## Current limitations

- No live worker process was killed or resumed. The restart observation is the replayer.
- The SDK test environment does not export history. The replay test builds history for this workflow.
- `GET /report` and `GET /evidence` are not implemented. Claim text and excerpts stay out of the API.
- Full tenant RBAC is the M4 gate. Registration already checks Keycloak issuer, audience, signature, expiry, and the developer and operator roles.
- These timings are one sample on one machine. They are not an availability or recovery objective.
