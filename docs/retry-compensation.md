# Retry classification and compensation — F25

Transient tool and model failures may be attempted again. A denied scope, an invalid model result, and an identity mismatch may not. [D40](decision-log.md#d40--f25-retry-classification-and-safe-compensation) records the limits.

## Classification and backoff

| Class | Attempt again |
|---|---|
| `tool_unavailable` | Yes |
| `model_unavailable` from a configured model call that failed | Yes |
| `model_unavailable` because no model is configured | No. The activity finishes and the workflow records `fail_closed` |
| `tool_denied`, `model_invalid`, `identity_mismatch` | No |

A retryable activity has three attempts. The intervals start at one second, double, and stop at a four-second ceiling. The command reconciler's eight attempts remain a separate budget for delivering the workflow start.

## Idempotent activity keys

A successful retrieval or generation is stored under a key made from the run ID, the step, and the frozen snapshot, question, and document list. A duplicate delivery with that key returns the stored observation and does not call the tool or the model again. A failed attempt is not stored, so a later attempt can still perform the call.

## Compensation

The selected partial side effect is evidence recorded by a search that never becomes a report. When generation then stops because no model is configured, the workflow compensates that run: the evidence retention changes from `workflow_history` to `compensated`, and any report stored for the run is dropped. Compensating the same run again does not change the record a second time.

This does not undo a hosted model charge. A generation that already returned is reused instead of being sent again. F25 does not add a product compensation table.
