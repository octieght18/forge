# F28 verification — 12 October 2026

F28 separates replay, retry, and re-execution and adds evaluation fixtures for a success, an uncertain result, and a failed step. [D43](decision-log.md#d43--f28-replay-and-evaluation-fixtures) records the boundary. The [mode guide](replay-modes.md) is the short description.

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/execution ./internal/mcp ./internal/model ./internal/httpapi ./cmd/forge-worker ./cmd/api
```

`TestEvaluationFixtures` uses the in-process fixture. The success case stores one evidence row with retention `workflow_history` and outcome `answered`. The uncertain case uses `architecture-notes`, returns `insufficient_evidence`, and stores no evidence. The failed step uses a snapshot the tool schema rejects, returns `tool_denied`, and does not call the tool.

`TestReplayDoesNotCallToolsAndReexecutionDoes` runs two workflow ids and sees the tool call count increase. It then replays a built history through `WorkflowReplayer`. Activity entries and tool calls stay at zero. HTTP rerun creates a new run linked by `rerun_of`. Repeating the idempotency key does not require Temporal. A second key while the result is not ready is 409. An operator rerun is 404. A process with no Temporal reader returns 503 for a new rerun.

## Material limits

- The SDK test environment does not export workflow history. Replay is tested with a history constructed for this workflow, not with a history downloaded from a server. No live Temporal dev server was started.
- `FORGE_DEBUG=true` is the debug default. Without it, a configured MCP command or model key is still used.
- Retry timing is the existing F25 policy. This ticket does not measure a new backoff.
- Re-execution can repeat a model or tool call. Replay cannot.
