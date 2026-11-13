# Cost analysis and operational evidence bundle — F56

[D71](decision-log.md#d71--f56-cost-and-operational-evidence-bundle) records the boundary. This page indexes measured cost, capacity, incidents, and backup/rollback evidence against the [product brief](product-brief.md).

## Per-request / per-workload cost (from F49)

| Unit | Value | Assumption |
|---|---|---|
| Illustrative cloud idle floor | **~$122.63 / month** | Unapplied EKS shape; us-east-1; price date 2026-10-10 |
| Model per research run | **$0** | Free Gemma route; no paid fallback |
| Infra $/request @ 1e6 req/mo full attribution | **~$0.000123** | Planning traffic, not observed cloud traffic |
| Infra $/request @ 10% attribution | **~$0.0000123** | Sensitivity case |
| Telemetry collector | **$0** | In-process only |
| Local kind/native | **$0 cloud** | Demonstrated path |

Details: [capacity-cost-model.md](capacity-cost-model.md), [validation/f49-capacity-cost.json](validation/f49-capacity-cost.json).

## Utilization scenarios

| Scenario | Utilization statement | Capacity note |
|---|---|---|
| Registration profile (F10) | 1–2 clients saturated the uncapped fixture to 1196–2148 req/s | Product enforces 200 req/s |
| Provisioning accept (F19) | 3 workloads / 3 kinds accepted | No higher published capacity |
| Research recovery (F29/F47) | Fixture/SDK recovery in tens of ms | Not live Temporal HA |
| Budget | Prefer free model + local stack; cloud floor ~$123/mo if applied | Spend approval required before apply |

## Linked operational evidence

### Simulated incidents (F48)

- [INC-SIM-001 database login loss](incidents/INC-SIM-001-database-login-loss.md)
- [INC-SIM-002 worker and tool outage](incidents/INC-SIM-002-worker-and-tool-outage.md)
- [INC-SIM-003 bad rollout and quota](incidents/INC-SIM-003-bad-rollout-and-quota.md)

### Backup / rollback measurements

| Item | Measurement | Doc |
|---|---|---|
| Dump | 61.758744 ms | [backup.md](backup.md) |
| Restore check | 71.794097 ms | [f46-validation.md](f46-validation.md) |
| Rollback detection | 292 ns | [rollback.md](rollback.md) |
| Pin write / whole call | 52.762 µs / 54.421 µs | F45 |
| F47 bad_rollout | 33 µs; idempotent second call | [failure-injection.md](failure-injection.md) |

### Benchmarks

- [performance-developer-benchmark.md](performance-developer-benchmark.md) (F54)
- [performance-targets.md](performance-targets.md) Balanced local (not production SLO)

## Retention, budget, capacity vs product brief

| Brief expectation | Bundle status |
|---|---|
| Usage and dated cost assumptions | Published in F49/F56 with price date |
| Backup/restore drill | F46 measured; not offsite/encrypted |
| Incident procedures | F48 runbooks + simulated reports |
| Quota/rate outcomes | F35/F47; per-process |
| Production recovery objectives | **Not selected**; G5 reviewed with demo limits ([g5-validation.md](g5-validation.md)) |
| Adoption feedback | Deferred to F62 |

## Limits

No terraform apply. No production SLO. G3–G5 reviewed with documented limits; G6 open. Incidents are labelled simulated.
