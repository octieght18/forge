# G4 verification — Tested multi-tenant trust controls

Reviewed on 13 November 2026. Suite re-run against tip `1dd1a91`; this receipt is published in the G4 commit. [D79](decision-log.md#d79--g4-multi-tenant-trust-controls) records the boundary. This is an owner evidence review with **PASS with limits**. It does not claim production isolation.

## Checklist

| Acceptance | Result | Evidence |
|---|---|---|
| Network/secret/retention and supply-chain controls have recorded tests | **PASS with limits** | F38–F40 tests re-run this session; network deny not observed on Kind; no live Kyverno; SBOM without signed image |
| Threat model explicitly separates demo evidence from production readiness | **PASS** | [threat-model.md](threat-model.md) demonstrated vs required; production gap list retained |
| Tenant/project isolation, OIDC/RBAC, quotas/rate limits, audit and metering are verified | **PASS** | F32–F36/F40 suite re-run this session (all packages `ok`) |

## Re-run suite (this session)

```text
go test -count=1 -timeout 180s ./internal/httpapi -run 'TestRoleMatrixAndDeniedTools|TestSingleProcessQuota|TestCrossTenantEndpointsStayHidden|TestRunHistoryAndResultHTTP|TestRegistrationHTTPPostgreSQL'
go test -count=1 -timeout 60s ./internal/auth -run 'TestAccessTokensAndKeyRotation'
go test -count=1 -timeout 60s ./internal/audit -run 'TestAuditOmitsSecretsAndDeduplicatesUsage'
go test -count=1 -timeout 60s ./internal/execution -run 'TestSanitizeHistoryOmitsPromptsCorpusAndTokens|TestPublicResultDropsUnsafeReasonAndNonIdentifiers'
go test -count=1 -timeout 30s ./internal/policy -run 'TestRepositoryManifestsAndNetworkAssumptions'
go test -count=1 -timeout 30s ./internal/mcp -run 'TestFixtureStoresProvenanceAndRedactsSecrets|TestDeniedToolNameDoesNotStartAProcess'
```

All six commands exited 0.

### Mapped controls

| Control | Result | Limit |
|---|---|---|
| Missing/forged/wrong-aud/expired token | Pass (`TestAccessTokensAndKeyRotation`, registration HTTP 401) | No revocation before 5-minute lifetime |
| Service / mixed role on product routes | Pass (403) | Not mesh SPIFFE |
| Cross-owner operator patch | Pass (404) | Org multi-tenant out of scope |
| Cross-tenant endpoints | Pass (`TestCrossTenantEndpointsStayHidden` → 404) | Demo tenant on legacy rows |
| Quota / rate limit | Pass (`TestSingleProcessQuota` → 429, `Retry-After: 1`) | Per-process only; not distributed |
| Audit omits secrets; usage dedupe | Pass | In-process meters; lost on exit |
| History/result/tool secret redaction | Pass | |
| Denied tool does not start process | Pass | |
| NetworkPolicy manifests | Pass as document check | `DeniedObserved=false`; Kind does not enforce |
| Supply-chain | Module SBOM (F39/F61) | `signed-image: not-run`; `clean-scan: not-claimed` when tools absent |

## Threat model separation

[threat-model.md](threat-model.md) keeps:

- **Demonstrated:** owner-private reads/writes; operator read-only cross-owner; other-tenant 404; service 403; MCP allow list; secret redaction; per-process quota; in-process audit.
- **Not claimed for production:** organization isolation; enforcing NetworkPolicy with observed deny; live Kyverno; reviewed image scan; cosign verify against operator key; token revocation; private model route; issued DNS+TLS; distributed quota/audit.

G3 is already Done (`1dd1a91`). This review closes G4 with the limits above. G5 and G6 stay open.

## Limits (explicit)

- No live NetworkPolicy deny packet was observed.
- Kyverno was not installed on a cluster; CI/policy-file checks remain the admission evidence.
- No image was scanned or signed in this review session.
- Quotas and audit are single-process demo controls.
- Closing G4 does not make the platform production-ready.

## Related docs

- [threat-model.md](threat-model.md), [f40-validation.md](f40-validation.md)
- [f32-validation.md](f32-validation.md)–[f36-validation.md](f36-validation.md), [f38-validation.md](f38-validation.md), [f39-validation.md](f39-validation.md)
- [g3-validation.md](g3-validation.md)
