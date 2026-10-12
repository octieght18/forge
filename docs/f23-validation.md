# F23 verification — 12 October 2026

F23 resolves the frozen run context and calls one read-only MCP tool. [D38](decision-log.md#d38--f23-context-resolution-and-mcp-tool-adapter) records the boundary. The [adapter guide](mcp-adapter.md) explains authorization, the fixture, and retention.

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/mcp ./internal/execution ./cmd/forge-worker ./cmd/forge-mcp-fixture
```

`TestDeniedScopeDoesNotCallTheTool` rejects a path-shaped document before the tool is called and stores no evidence. `TestFixtureStoresProvenanceAndRedactsSecrets` reads one synthetic passage from the fixture, stores its snapshot, passage ID, excerpt hash, and `workflow_history` retention, and removes a bearer token and database URL from the tool error. `TestDeliveryUsesOneWorkflowID` refuses a document outside the version allow list before a second workflow start. The existing workflow test still finishes `fail_closed` with `model_unavailable` when no corpus command is configured.

No model call, public run route, or product evidence table was added.

Documentation checks reported 58 Markdown documents and 417 relative links. The PowerShell checker parsed 8 scripts and 30 documented PowerShell blocks.
