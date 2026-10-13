# F42 verification — 13 October 2026

F42 adds an Envoy gateway manifest and records the traffic path that actually exists. [D57](decision-log.md#d57--f42-gateway-dns-and-tls) records the boundary. The limits are in [gateway.md](gateway.md).

The manifest was not applied. No domain was bought. No certificate was issued. No load balancer was created. TLS was not verified. The local substitute remains `http://127.0.0.1:8081` through the kind NGINX proxy.

`TestRegistrationHTTPPostgreSQL` passed on the disposable PostgreSQL 16.15. A workload list without a token returned 401 with `WWW-Authenticate: Bearer`. A developer token created a workload and returned 201. That is the authenticated and unauthenticated result. It is not a TLS result.

`TestRepositoryManifestsAndNetworkAssumptions` passed, so the Envoy pod is not privileged. The gateway document has a ClusterIP Service, a `/api/` route, and a 200 token local rate limit. It has no private key.

Checked with Go 1.27.1:

```text
go test -count=1 -timeout 180s ./internal/httpapi -run 'TestRegistrationHTTPPostgreSQL'
go test -count=1 -timeout 30s ./internal/policy -run 'TestRepositoryManifestsAndNetworkAssumptions|TestGatewayIsLocalAndUnauthenticatedPathStaysOnTheAPI'
```
