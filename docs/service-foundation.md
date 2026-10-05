# Go service foundation (F03)

The owner accepted the standard library HTTP server, `slog` JSON logging, loopback port 8081, package boundaries and operational conventions on 5 October 2026. This service provides process health and establishes the lifecycle used by later API implementation.

## Run and check

Install Go 1.27.1 from the [official downloads](https://go.dev/dl/). The HTTP service uses the standard library; F04 adds a pinned JSON Schema validator and `go.sum`. The toolchain version is shared by local builds and CI.

```sh
go run ./cmd/api
```

Visit `http://127.0.0.1:8081/healthz` or `/readyz`. Stop with Ctrl+C (SIGTERM also works on Linux). Use the compiled executable when testing process signals; `go run` introduces a parent tool process.

```sh
go build -o tmp/forge-api ./cmd/api
go vet ./...
go test -timeout 60s ./...
go build ./...
```

On Windows, use `go build -o tmp/forge-api.exe ./cmd/api`. Run `gofmt -l .`; a clean result has no filenames. On a machine with a supported C compiler, also run `go test -race -timeout 60s ./...`. CI runs build, formatting, vet and tests on Windows/Linux, plus race detection on Linux. Workflow actions are pinned to verified commit hashes and have read-only repository permissions.

For this PC, the verified portable toolchain is in `C:\Users\Owner\Documents\Codex\2026-10-04\in\tools\go`. A PowerShell session can use it without changing global configuration:

```powershell
$env:Path = 'C:\Users\Owner\Documents\Codex\2026-10-04\in\tools\go\bin;' + $env:Path
Set-Location 'C:\Users\Owner\Downloads\Code\forge'
go run ./cmd/api
```

## Boundaries and dependency injection

| Package | Responsibility |
| --- | --- |
| `cmd/api` | Composition root: load environment, create logger/handler/server, bind the listener, receive signals and choose process exit status. |
| `internal/config` | Parse and validate configuration before serving; environment lookup is injected. |
| `internal/httpapi` | Health routes, JSON error responses, server-generated correlation IDs, safe logging and panic handling. |
| `internal/service` | HTTP transport timeouts, readiness state, request base context and bounded shutdown; handler/logger/readiness are injected. |

Use constructors and explicit dependencies. Add domain/application and adapter packages when their tickets introduce real behavior; do not create empty abstractions or put Temporal/PostgreSQL access inside transport handlers. There are no mutable global application dependencies. Each server owns one listener and runs once.

Internal operations wrap errors with `%w`; callers can use `errors.Is`/`errors.As`. HTTP handlers map failures to an explicit safe public message and stable code. Never serialize raw dependency errors or panic values.

```json
{"error":{"code":"not_ready","message":"Service is not ready"},"request_id":"server-generated-32-hex-characters"}
```

Known codes are `not_found` (404), `method_not_allowed` (405), `not_ready` (503), `unavailable` (503, request-ID generation failure) and `internal_error` (500). `X-Request-ID` matches the response/log correlation value; incoming IDs are ignored. Logs contain status and correlation, without request bodies, credentials, query strings or arbitrary panic contents. Responses use JSON, `Cache-Control: no-store` and `X-Content-Type-Options: nosniff`. A panic before output yields a safe 500; a panic after output aborts the connection because partial output cannot be replaced reliably.

## Configuration and lifecycle

| Environment variable | Default |
| --- | --- |
| `FORGE_HTTP_ADDR` | `127.0.0.1:8081` |
| `FORGE_HTTP_READ_HEADER_TIMEOUT` | `5s` |
| `FORGE_HTTP_READ_TIMEOUT` | `10s` |
| `FORGE_HTTP_WRITE_TIMEOUT` | `15s` |
| `FORGE_HTTP_IDLE_TIMEOUT` | `60s` |
| `FORGE_SHUTDOWN_TIMEOUT` | `10s` |

Addresses must use a literal loopback IPv4/IPv6 address and port 1–65535; wildcard, hostname and remote binds are rejected. Durations must be positive Go durations; header timeout cannot exceed the total request read timeout. Header size is capped at 1 MiB. Invalid configuration/listen failure exits with status 1. Future container deployment must explicitly revisit the loopback restriction.

These defaults are operational guardrails, not F08 performance targets. Per [Go's HTTP server semantics](https://pkg.go.dev/net/http#Server), read timeout bounds reading the full request, write timeout bounds transport writes, and idle timeout bounds the wait for a keep-alive request. They do not interrupt arbitrary CPU work or automatically give handler code an execution deadline. Future dependency calls must use request contexts plus their own bounded timeouts; request body limits belong with the product API contracts.

`GET` and `HEAD /healthz` indicate that the process can respond. `/readyz` returns 200 while serving and 503 before serving or during draining. Readiness presently checks this process only; it makes no claim about PostgreSQL, Temporal, Keycloak or model availability. Both endpoints are public and contain no product data. All other paths return 404; no workload endpoint is exposed before OIDC/ownership integration.

On shutdown, readiness becomes false, new connections stop, and active requests have the configured grace period to finish. The process signal context is deliberately separate from request contexts. If grace expires, cancel request contexts and close active connections; report failure with exit status 1. Client disconnects also cancel request contexts. Handlers must cooperate with cancellation; Go cannot forcibly stop a goroutine. Successful shutdown exits 0. Unexpected listener failure is reported and releases active connections. Restartability inside the same process, streaming/hijacked connections and WebSockets are outside this foundation.

## Evidence

See [F03 validation](f03-validation.md) and [CI](../.github/workflows/ci.yml). Future API schemas, identity configuration and workflow/storage integration remain governed by [F02 architecture](architecture.md).
