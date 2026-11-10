# Final platform review and demo rehearsals — F60

[D75](decision-log.md#d75--f60-final-platform-review) records the boundary. Gates G3–G6 stay open.

## FR01–FR12 acceptance matrix

| ID | Capability | Result | Evidence |
|---|---|---|---|
| FR01 | Register and version AI workload | **Pass** (demo) | F06/F09 seven ops; contract tests |
| FR02 | Declare tools, permissions, limits, policy | **Pass** (demo) | `research.v1`; only corpus tools; small-v1 unchanged |
| FR03 | Provision isolated environment | **Pass** with limits | F16/F19 `202 accepted`; operator apply; no developer kubeconfig; Kind NetworkPolicy unenforced |
| FR04 | Asynchronous long-running workflows | **Pass** with limits | F22–F27; SDK/Temporal worker path; F29/F47 without live kill |
| FR05 | Versioned authenticated API | **Pass** | OpenAPI; Keycloak; F09; service token 403 |
| FR06 | Quotas and rate limits | **Pass** with limits | F35/F47; per-process only; not distributed |
| FR07 | Usage and execution history | **Pass** with limits | F27 history/result; F36 meters in-process (lost on exit) |
| FR08 | Retry safely after failures | **Pass** with limits | F25/F29/F47; 0 duplicate side effects in samples |
| FR09 | Roll out versions | **Pass** with limits | F44/F45 pin canary; cloud promote refused; no live staging window |
| FR10 | Debug via replay/rerun | **Pass** with limits | F28/F30; replay≠rerun; no full UI |
| FR11 | Audit trail | **Pass** with limits | F36 actor/tenant/path/outcome; no token/prompt storage; in-process |
| FR12 | Documented self-service deployment/onboarding | **Pass** with limits | Quickstart + F50 cloud guide; F53: native Keycloak not re-run on cloud VM; human worksheet null |

**Deferred / open:** production TLS/DNS, applied EKS, live Argo, signed images, org isolation, distributed quota, G3–G6.

## Demo rehearsal notes (written)

1. **Architecture (60s):** Compose API + PG + Temporal + Keycloak + controller; cite diagrams.
2. **Failure/recovery (60s):** Walk F47 table; emphasize replay 0 tool calls; backup excludes Temporal history.
3. **Security limits (60s):** Owner matrix, tenant 404, service 403; say G3/G4 open; NetworkPolicy not enforced.
4. **Cost (45s):** Free model $0; ~$123/mo illustrative unapplied floor; no invoice.
5. **Tradeoffs (45s):** In-process audit; per-process limits; honesty over unverifiable claims.
6. **Close (30s):** Point to article, resumes, release checklist; do not claim production ready.

Critical issues found in this review: stale quickstart/api-examples claims were already corrected in F53. No additional critical code defects blocked the matrix. Remaining work is documented below rather than overstated as ready.

## Remaining work (not closed)

- Close G3–G6 with owner acceptance after stronger evidence.
- Live Temporal worker-kill test; enforcing NetworkPolicy observation; Kyverno install; image sign/scan.
- terraform apply only with spend approval; DNS/TLS; collect external developer feedback (F62 template).
