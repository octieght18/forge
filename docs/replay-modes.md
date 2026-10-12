# Replay, retry, and re-execution — F28

The [workflow and replay guide](workflow-replay-guide.md) is the operator procedure and the recorded failure timeline. This page is the mode boundary.

A failed step can be inspected in three different ways. They are not interchangeable. [D43](decision-log.md#d43--f28-replay-and-evaluation-fixtures) records the boundary.

| Mode | What runs | Side effects |
|---|---|---|
| Replay | Workflow code against Temporal history | No model call and no MCP call. Activity functions are not entered. |
| Retry | The same activity, up to three attempts | A retryable class may call the tool or model again. A committed activity returns its stored result. |
| Re-execution | `POST /api/v1/runs/{run_id}/rerun` | A new run and workflow ID `forge-run/<new UUID>`, linked by `rerun_of`. This can call the fixture or the configured tool. |

Retry is the F25 policy: `tool_unavailable` and a failed call to a configured model, starting at one second and doubling to a four-second ceiling. `tool_denied`, `model_invalid`, and identity mismatches are not retried.

Re-execution requires a terminal Temporal result for a new idempotency key. The same key returns the accepted run without asking Temporal again. An operator cannot rerun another owner's run. Without `TEMPORAL_ADDRESS`, a new rerun returns 503.

## Debug fixtures

`FORGE_DEBUG=true` selects the in-process corpus fixture and the fixture model. A configured `FORGE_MCP_COMMAND` or `FORGE_MODEL_API_KEY` is not used on that path. Replay uses `worker.WorkflowReplayer` and does not start either one.

## Evaluation cases

The SDK test suite covers three results:

- Success cites one `platform-brief` passage, outcome `answered`, retention `workflow_history`.
- An uncertain result uses a document the fixture does not contain. The outcome is `insufficient_evidence` and no evidence row is stored.
- A failed step uses a snapshot the invocation schema rejects. The decision is `fail_closed` with `tool_denied`. The tool function is not called.

The test environment does not export history. The replay test builds a history whose activity ids are the Temporal command event ids and replays it. Activity entry and tool-call counters stay at zero. A second `ExecuteWorkflow` with a new run id is the re-execution check and does call the tool. No live Temporal dev server was started.
