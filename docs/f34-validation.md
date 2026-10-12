# F34 verification — 12 October 2026

F34 checks the role matrix at the API and stops a denied tool before a process starts. [D49](decision-log.md#d49--f34-rbac-and-tool-authorization) records the boundary. The matrix is [authorization.md](authorization.md).

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/mcp ./internal/httpapi ./internal/auth
```

`TestRoleMatrixAndDeniedTools` shows an operator patch of another owner's workload is 404, a developer read of telemetry is 403, an operator read of telemetry is 200, a service token is 403, and a token that mixes developer, operator, and service is 403. A version that names `corpus.write` is 422. `TestDeniedToolNameDoesNotStartAProcess` points the MCP command at a script that would create a file, calls `corpus.write`, and leaves that file absent. `TestDeniedScopeDoesNotCallTheTool` still does not call the tool for a document outside the frozen list. `TestCrossTenantEndpointsStayHidden` still returns 404 for another tenant's run id on cancel, history, result, and rerun.

The tool boundary does not repeat the tenant query. Tenant scope is the API admission of the run. G3 stays open.
