# F25 verification — 12 October 2026

F25 classifies activity failures, bounds retries, reuses a committed activity, and compensates evidence left after a run stops. [D40](decision-log.md#d40--f25-retry-classification-and-safe-compensation) records the boundary. The [retry guide](retry-compensation.md) lists the classes and the backoff.

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/execution ./internal/mcp ./internal/model ./cmd/forge-worker
```

`TestRetryReusesWorkAndCompensatesPartialEvidence` checks that `tool_unavailable` and a failed model call are retryable, and that `tool_denied` and `model_invalid` are not. The policy is three attempts, starting at one second and doubling to a four-second ceiling. A tool fails on the first call and succeeds on the second. The workflow then stops with `fail_closed` because no model is configured. The stored passage retention is `compensated`. A duplicate retrieval does not call the tool again, and a second compensation leaves that retention in place.

The cited-report workflow still completes `answered` and is not compensated. No hosted model charge is reversed by this ticket.

Documentation checks reported 62 Markdown documents and 435 relative links. The PowerShell checker parsed 8 scripts and 30 documented PowerShell blocks.
