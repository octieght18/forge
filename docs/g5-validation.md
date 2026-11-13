# G5 verification — Operational deployment and recovery

Reviewed on 13 November 2026. [D80](decision-log.md#d80--g5-operational-deployment-and-recovery) records the boundary. This is an owner evidence review with **PASS with limits**. It does not claim an applied cloud environment or production recovery SLOs.

## Checklist

| Acceptance | Result | Evidence |
|---|---|---|
| Promotion, bad-release rollback and backup/restore drills have measured results | **PASS** | Re-run this session (below) + published F44/F45 procedures |
| Failure suite, three simulated incidents, runbooks and cost/capacity model are documented | **PASS** | F47–F49 docs + failure suite re-run this session |
| Terraform and GitOps deployment path is reproducible for one cloud | **PASS with limits** | Guide + GitOps/gateway/promote tests; `terraform apply` / live Argo / DNS/TLS not run |

## Re-run this session

### Promotion and rollback (Windows)

```text
go test -count=1 ./internal/delivery ./cmd/forge-promote
go test -count=1 -v -run 'TestRollback' ./internal/delivery
```

Both packages/`TestRollback*` passed. Timer lines printed `0s` at process resolution; published F45 samples remain the timed reference (detection 292 ns, pin write 52.762 µs, call 54.421 µs).

### Backup / restore (WSL, pg_dump 18.6)

```text
go test -count=1 -v -run TestDisposableDumpRestoresTheWorkload ./internal/backup
```

`dump=65.985041ms` `restore=150.327061ms` — PASS. Disposable DB only (`forge_http_*`); native product DB name refused by the test.

### Failure injection (Windows, admin URL set)

```text
go test -count=1 -timeout 120s -v ./internal/failure
```

| Scenario | duration_us | side_effects |
|---|---|---|
| worker_loss | 39784 | 0 |
| tool_model_outage | 24047 | 1 |
| bad_rollout | 2082 | 1 |
| database_outage | 25223 | 1 |
| quota_exhaustion | 19210 | 1 |

### GitOps / gateway docs path

```text
go test -count=1 -timeout 30s ./internal/policy -run 'TestGitOpsPinsExistingDigests|TestRepositoryManifestsAndNetworkAssumptions|TestGatewayIsLocalAndUnauthenticatedPathStaysOnTheAPI'
python scripts/check-docs.py
```

Policy tests ok. Docs checker: 141 Markdown documents / 991 relative links (pre-receipt count; re-run after this file lands).

## Documentation pack (AC2)

| Artifact | Path |
|---|---|
| Failure suite | [failure-injection.md](failure-injection.md), [f47-validation.md](f47-validation.md) |
| Simulated incidents | [incidents/](incidents/) INC-SIM-001…003 |
| Runbooks | [runbooks/](runbooks/) |
| Cost / capacity | [capacity-cost-model.md](capacity-cost-model.md), [validation/f49-capacity-cost.json](validation/f49-capacity-cost.json) |
| Ops index | [operational-evidence-bundle.md](operational-evidence-bundle.md) |
| Cloud guide | [cloud-deployment-guide.md](cloud-deployment-guide.md), [f50-validation.md](f50-validation.md) |

Incidents remain labelled simulated. Cost model remains dated/illustrative (no invoice, no apply).

## Cloud path limits (AC3)

Documented and locally tested:

- EKS Terraform module docs (`terraform-eks.md`)
- GitOps manifests / pin digest tests
- Promote copies local→staging and **refuses** cloud/eks/gke
- Gateway stays local/unprivileged assumptions

**Not run this review:** `terraform apply` / `destroy`, Argo CD install or live reconcile, DNS/TLS issuance, paid cloud spend.

## Limits

- No applied EKS or live GitOps controller.
- Backup dump still excludes Temporal history and in-process evidence/audit.
- Rollback does not reverse migrations or unsend model calls.
- Failure suite uses fixtures / disposable roles; not a multi-AZ production failover test.
- G6 stays open.

## Related

- [promotion.md](promotion.md), [rollback.md](rollback.md), [backup.md](backup.md)
- [f44-validation.md](f44-validation.md)–[f50-validation.md](f50-validation.md)
- [g3-validation.md](g3-validation.md), [g4-validation.md](g4-validation.md)
