# F40 verification — 13 October 2026

F40 records the security checks that ran and the checks that did not. [D55](decision-log.md#d55--f40-tenant-security-validation) records the boundary. The updated model is [threat-model.md](threat-model.md). G3 stays open. G4 is not marked done. This is not a production-readiness claim.

The database for the HTTP cases was a disposable PostgreSQL 16.15 on this machine. It was not the retained product database and not the CI PostgreSQL 18.6.

| Case | Result |
|---|---|
| Auth: missing bearer token on `GET /api/v1/workloads` and a non-JSON body without a token | Pass. `TestRegistrationHTTPPostgreSQL` expects 401 and `WWW-Authenticate: Bearer` |
| Auth: wrong issuer, wrong audience, expired token, forged signer, workload bearer token | Pass. `TestAccessTokensAndKeyRotation` |
| RBAC: operator patch of another owner's workload | Pass. `TestRoleMatrixAndDeniedTools` expects 404 |
| RBAC: developer telemetry | Pass. 403 |
| RBAC: service token and mixed service role | Pass. 403 |
| RBAC: `corpus.write` on a version | Pass. 422 |
| RBAC: another tenant | Pass. `TestCrossTenantEndpointsStayHidden` |
| Quota: third request in a window of 2 | Pass. `TestSingleProcessQuota` expects 429, `Retry-After: 1`, and `rate_limited` |
| Secret redaction: audit event | Pass. `TestAuditOmitsSecretsAndDeduplicatesUsage` and the RBAC audit assertion. The event JSON has no bearer token |
| Secret redaction: history and result | Pass. `TestSanitizeHistoryOmitsPromptsCorpusAndTokens` and `TestPublicResultDropsUnsafeReasonAndNonIdentifiers` |
| Secret redaction: tool error | Pass. `TestFixtureStoresProvenanceAndRedactsSecrets` |
| Network deny | Not run. `TestRepositoryManifestsAndNetworkAssumptions` passed as a document check and recorded `DeniedObserved=false`. No packet was dropped |

Checked with Go 1.27.1:

```text
go test -count=1 -timeout 180s ./internal/httpapi -run 'TestRoleMatrixAndDeniedTools|TestSingleProcessQuota|TestCrossTenantEndpointsStayHidden|TestRunHistoryAndResultHTTP|TestRegistrationHTTPPostgreSQL'
go test -count=1 -timeout 60s ./internal/auth -run 'TestAccessTokensAndKeyRotation'
go test -count=1 -timeout 60s ./internal/audit -run 'TestAuditOmitsSecretsAndDeduplicatesUsage'
go test -count=1 -timeout 60s ./internal/execution -run 'TestSanitizeHistoryOmitsPromptsCorpusAndTokens|TestPublicResultDropsUnsafeReasonAndNonIdentifiers'
go test -count=1 -timeout 30s ./internal/policy -run 'TestRepositoryManifestsAndNetworkAssumptions'
go test -count=1 -timeout 30s ./internal/mcp -run 'TestFixtureStoresProvenanceAndRedactsSecrets|TestDeniedToolNameDoesNotStartAProcess'
```

Each command exited 0.
