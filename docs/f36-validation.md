# F36 verification — 12 October 2026

F36 records audit events and deduplicated usage. [D51](decision-log.md#d51--f36-audit-and-usage-metering) records the boundary.

An audit event has actor, tenant, operation, timestamp, target, and outcome. The actor is the token subject. The target is the request path. The body, the bearer token, the prompt, and corpus text are not fields. Text that looks like a bearer token or a database URL is stored as `redacted`.

Model usage is prompt tokens and completion tokens from the report. Tool usage is one call. Both are stored once per activity idempotency key. A second note for the same key does not add.

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/audit ./internal/execution ./internal/httpapi
```

`TestFixtureUsageReconcilesOnce` runs the fixture model and checks the meter against that report's prompt and completion tokens and one tool call. `TestRoleMatrixAndDeniedTools` then reads one workload and stores actor `a`, the demo tenant, operation `GET`, outcome `success`, and the workload path. The event JSON does not contain the access token.

The meter is in the API and worker process. It is not a shared ledger across processes. Fixture executions are the reconciliation source used here. No live OpenRouter call was made.
