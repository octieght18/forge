# Temporal worker — F22

PostgreSQL stores the run and the start or cancel command. Temporal stores execution history. The dispatcher delivers a command and returns. It does not copy Temporal's status back into a product table. [D37](decision-log.md#d37--f22-temporal-execution-workers) records that split.

The workflow ID is the run's existing `forge-run/<run UUID>`. Reuse of that ID is rejected, so a second start describes the original execution and does not create another one. Cancel of a running execution requests cancellation. Cancel of a closed execution is already handled, because completion can win. Cancel of a missing execution stays undelivered for investigation.

## Local server

The dev server is a separate process from the API. From Windows, with the Temporal CLI installed:

```powershell
./scripts/temporal-dev.ps1
```

That listens on `127.0.0.1:7233` and keeps its database file under the temporary directory. The default namespace is `default`. The task queue is `forge-research`.

In another terminal, build and run the worker and the dispatcher. The dispatcher uses the private dispatcher database role, not the API role:

```text
go run ./cmd/forge-worker
go run ./cmd/forge-dispatch
```

`TEMPORAL_ADDRESS` and `TEMPORAL_NAMESPACE` override the defaults. `FORGE_DISPATCHER_DATABASE_URL` is required by the dispatcher and is not injected into the API.

## Workflow and activities

`research.v1` checks that Temporal's workflow ID matches the product workflow ID and the run ID. It passes the run, workflow, and command IDs into each activity. When `FORGE_MCP_COMMAND` is set, retrieval calls [corpus.search](mcp-adapter.md) after the frozen document scope is checked. Otherwise retrieval does not start a tool. The generation activity returns `model_unavailable`. The workflow then completes with decision `fail_closed`. That completion means the control-plane decisions finished. It is not a research report and it is not written to PostgreSQL.

MCP adapters and model calls belong to later tickets. A missing historical execution is not started again when the command is a cancel. A start whose execution is already present, running or closed, reuses that ID.

## Limits

The SDK test suite proved the workflow and the delivery rules. This session could not open the WSL Docker socket, so a live dev server was not started here. The native `local-stack.ps1` path still does not launch Temporal or the dispatcher. Public run routes remain absent. There is no second execution status column.
