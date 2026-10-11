# F17 verification — 11 October 2026

F17 records provisioning duration, success and failure counts, and reconciliation retry reasons with OpenTelemetry. [D31](decision-log.md#d31--f17-in-process-opentelemetry-views) records the boundary. No collector or dashboard is added. The controller-runtime metrics server stays unbound, and the controller process does not gain a scrape Service.

The API view is `GET /api/v1/telemetry` for an operator. It returns outcome counts, durations, and recent spans carrying request and operation IDs. A developer receives 403.

Checked with Go 1.27.1. Windows ran the packages without a database. A temporary PostgreSQL 18 on Ubuntu then ran `TestProvisioningOperations` with `FORGE_REQUIRE_DB_TESTS=1`.

```text
go test -count=1 ./internal/telemetry ./internal/environment ./internal/httpapi
go test -count=1 -run TestProvisioningOperations ./internal/httpapi
```

`internal/telemetry` covers ready, failed, and unspecified provisioning samples, retry and failure reasons, collapse of arbitrary reason text, and rejection of unsafe span identities. `internal/environment` covers a ready reconcile recording success and a pending retry, and an owner mismatch recording `OwnerMismatch`. `internal/httpapi` covers `traceparent` propagation onto the span that also carries the server request ID and operation ID. The PostgreSQL test records failed, canceled, and timed-out provisioning outcomes, shows them on the operator telemetry view, and rejects a developer read with 403.

Documentation checks reported 47 Markdown documents and 344 relative links. The PowerShell checker parsed 7 scripts and 29 documented PowerShell blocks.
