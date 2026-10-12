# MCP tool adapter — F23

The worker resolves a frozen run context and calls `corpus.search` only after that context is valid. [D38](decision-log.md#d38--f23-context-resolution-and-mcp-tool-adapter) records the boundary. Model execution stays a later ticket.

## Context

The dispatcher copies the run question, the version snapshot, the version's document allow list, and the passage limit into the workflow input. A document that is not on the version allow list fails delivery before Temporal is asked to start another execution. The retrieval activity checks the same scope again before it starts a process.

`corpus.search` arguments are the frozen question, snapshot, document IDs, and limit. A path, URL, command, or any other tool name is rejected by the invocation schema. The rejection does not start the tool.

## Adapter and test tool

`FORGE_MCP_COMMAND` is one executable path. The worker starts it with a clean environment that keeps `PATH` and drops the worker's other variables. The protocol is newline-delimited JSON-RPC: `initialize`, then `tools/call`. [cmd/forge-mcp-fixture](../cmd/forge-mcp-fixture) is the deterministic tool. It returns one synthetic `platform-brief` passage for `corpus.search` and the same passage for `corpus.read`. It returns no passage for any other document.

Tool errors are scanned for bearer tokens, database URLs, and `password=` text. Those spans are replaced with `[redacted]` before the error leaves the activity.

## Evidence

Each accepted passage is stored in the worker process with the run ID, workflow ID, snapshot, document, passage, SHA-256 of the excerpt, and retention `workflow_history`. That means the record lives with the Temporal history for the namespace retention period. F23 does not add a product evidence table or a separate expiry clock. A passage whose document was not in the authorized request is dropped and is not stored.

Without `FORGE_MCP_COMMAND`, retrieval does not start a tool. Generation still returns `model_unavailable`, and the workflow still completes `fail_closed`.
