# F26 verification — 12 October 2026

F26 sets workflow and activity deadlines, heartbeats the tool and model calls, and records a terminal state when a run is canceled. [D41](decision-log.md#d41--f26-timeout-cancellation-and-long-running-status) records the boundary. The [timeout guide](execution-timeouts.md) lists the limits and the incomplete-compensation case.

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/execution ./internal/mcp ./internal/model ./cmd/forge-worker
```

`TestCancellationCompensatesOrExplainsTheGap` blocks inside the tool until the workflow is canceled. The activity context is canceled, compensation runs on a disconnected context, and the recorded state is `canceled` with either `compensation_completed` or `compensation_incomplete`. The workflow execution timeout is 2 minutes, the activity start-to-close timeout is 30 seconds, and the heartbeat timeout is 10 seconds. The cited-report and retry tests still pass, including a heartbeat before retrieval and generation.

No public status route was added. The in-process terminal list is test evidence. A restarted worker reads the same outcome from Temporal history.

Documentation checks reported 64 Markdown documents and 443 relative links. The PowerShell checker parsed 8 scripts and 30 documented PowerShell blocks.
