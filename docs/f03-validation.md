# F03 validation

Date: 5 October 2026

| Acceptance criterion | Implementation and evidence |
| --- | --- |
| Package boundaries, dependency injection, structured error conventions | `cmd/api`, `internal/config`, `internal/httpapi`, `internal/service`; injected environment/logger/handler/readiness/listener; safe JSON codes and request correlation; configuration and response/panic tests. |
| Timeouts, cancellation, graceful shutdown, health endpoints | Configurable bounded HTTP transport, process health/readiness, separate signal/request contexts and bounded draining. Real TCP/HTTP tests verify health/HEAD, client disconnect, graceful completion, forced cancellation, slow headers, slow request bodies, late response writes and listener failure. |
| CI build, formatting, vet and meaningful checks | [Go CI](../.github/workflows/ci.yml) on Windows/Linux; formatting, vet, tests and build; Linux race detection; pinned actions and read-only permissions. |

Local Windows verification uses the official Go 1.27.1 amd64 archive with its published SHA-256 verified before extraction. `gofmt`, `go vet`, `go test -timeout 60s ./...` and `go build ./...` passed. Tests use actual local sockets for lifecycle/deadline behavior and channel synchronization for active requests. Small deadline fixtures exercise timeout behavior; they are not benchmark/SLO measurements. The CI workflow supplies Linux and race checks; the ticket links the resulting run after publication.

Limitations: this ticket adds no product API/storage/workflow/OIDC/model integration. Health is process-only and access is restricted to loopback. Windows lacks a C compiler in the local environment, so local race detection is not claimed. Read/write transport deadlines do not forcibly interrupt handler computation; handlers must honor cancellation. Full dependency availability, authentication/ownership, OpenAPI contracts, tracing and numerical F08 thresholds remain subsequent work.
