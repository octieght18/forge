# F21 verification — 11 October 2026

F21 adds the execution, tool, and evidence payloads to the OpenAPI contract and checks them with the existing offline validator. [D35](decision-log.md#d35--f21-execution-tool-and-evidence-contracts) records the boundary. The [contract guide](execution-contract.md) explains request, execution, workflow decisions, activities, MCP calls, typed failures, evidence provenance, and uncertain reports.

The G2 provisioning gate remains on the Backlog. This design uses the accepted research contract and does not mark that gate complete.

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/contract
```

The manifest validates `MCPInvocation`, an empty `MCPResult`, a succeeded retrieval, a `tool_denied` retrieval, a deterministic workflow decision, and an `insufficient_evidence` report. Rejection tests cover a path argument, a shell tool, a workflow decision marked nondeterministic, a workflow excerpt, a failed activity without a class, a successful activity that also carries a failure, and an activity step named `workflow`.

No run HTTP route, Temporal worker, MCP process, or database migration is added.

Documentation checks reported 53 Markdown documents and 386 relative links. The PowerShell checker parsed 7 scripts and 29 documented PowerShell blocks.
