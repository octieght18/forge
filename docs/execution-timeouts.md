# Timeout, cancellation, and long-running status — F26

A running research workflow can be canceled, and its end state stays in Temporal history across a worker restart. [D41](decision-log.md#d41--f26-timeout-cancellation-and-long-running-status) records the limits. PostgreSQL still stores only the run and the cancel command.

## Timeouts and heartbeats

| Limit | Value |
|---|---|
| Workflow execution | 2 minutes |
| Activity start-to-close | 30 seconds |
| Activity heartbeat | 10 seconds, recorded every 2 seconds while a tool or model call is in progress |
| Retryable attempts | 3, from the F25 policy |

Retrieval and generation record a heartbeat before the external call. The tool process is started with the activity context, so a canceled or expired context stops that process. A configured model call uses the same context.

## Cancellation

Canceling the workflow cancels the activity context. The tool or model returns that cancellation instead of being classified as a retryable dependency failure. The workflow then uses a disconnected context to compensate evidence left behind by a search and to record a terminal state.

The recorded states are `complete`, `fail_closed`, `canceled`, and `timed_out`. `timed_out` is reserved for a deadline that ends the workflow; this implementation records `canceled` when the caller cancels it. Compensation is `compensation_completed` when that cleanup activity finishes and `compensation_incomplete` when it does not. A canceled run stays canceled either way. Incomplete compensation means the evidence may still be marked `workflow_history` and a partial report may still be in the worker process. The in-process list is not the restart record. Temporal history is.

## Limits

F27 adds an authenticated history and result read over Temporal. A worker restart still observes the same workflow through Temporal, not through a product table. Compensation does not reverse a hosted model charge.
