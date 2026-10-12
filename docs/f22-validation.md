# F22 verification — 12 October 2026

F22 connects the existing start/cancel command to a Temporal workflow and worker. [D37](decision-log.md#d37--f22-temporal-execution-workers) records the boundary. The [worker guide](temporal-worker.md) explains the local dev server, the workflow ID, and the activity limit.

Checked with Go 1.27.1 and `go.temporal.io/sdk` v1.35.0:

```text
go test -count=1 ./internal/execution ./cmd/forge-worker ./cmd/forge-dispatch
```

`TestWorkflowPropagatesCorrelationAndStopsBeforeModel` runs the workflow in the SDK test suite. The result keeps the run ID, the `forge-run/<run UUID>` workflow ID, and the command ID, and finishes `fail_closed` with `model_unavailable`. `TestDeliveryUsesOneWorkflowID` starts once, treats a second start as the same execution, cancels that execution, rejects a mismatched workflow ID, and leaves a missing execution undelivered.

`sigs.k8s.io/controller-runtime` stayed at v0.25.2. No public run route, MCP process, model call, or product execution-status column was added. The WSL Docker socket was not open to this session, so `scripts/temporal-dev.ps1` was parsed and not executed against a live server.

Documentation checks reported 56 Markdown documents and 404 relative links. The PowerShell checker parsed 8 scripts and 30 documented PowerShell blocks.
