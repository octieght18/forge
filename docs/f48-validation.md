# F48 verification — 10 November 2026

F48 publishes operator runbooks and three simulated incident reports. [D63](decision-log.md#d63--f48-incident-runbooks-and-reports) records the boundary.

## Delivered

| Artifact | Path |
|---|---|
| Runbook index | [runbooks/README.md](runbooks/README.md) |
| Provisioning stuck | [runbooks/provisioning-stuck.md](runbooks/provisioning-stuck.md) |
| Execution stuck | [runbooks/execution-stuck.md](runbooks/execution-stuck.md) |
| Auth failures | [runbooks/auth-failures.md](runbooks/auth-failures.md) |
| Data restore | [runbooks/data-restore.md](runbooks/data-restore.md) |
| Incident index | [incidents/README.md](incidents/README.md) |
| INC-SIM-001 | [incidents/INC-SIM-001-database-login-loss.md](incidents/INC-SIM-001-database-login-loss.md) |
| INC-SIM-002 | [incidents/INC-SIM-002-worker-and-tool-outage.md](incidents/INC-SIM-002-worker-and-tool-outage.md) |
| INC-SIM-003 | [incidents/INC-SIM-003-bad-rollout-and-quota.md](incidents/INC-SIM-003-bad-rollout-and-quota.md) |

Corrective actions cite F47 scenarios `worker_loss`, `tool_model_outage`, `bad_rollout`, `database_outage`, and `quota_exhaustion` plus F45/F46/F29 numbers. Reports are labelled simulated.

## Checks

```text
python3 scripts/check-docs.py
go test -count=1 -timeout 180s -run TestFailureInjection ./internal/failure
```

Documentation check and the non-database F47 scenarios ran in this session. Database and quota scenarios still need `FORGE_TEST_ADMIN_DATABASE_URL` (or `FORGE_REQUIRE_DB_TESTS=1` with a disposable admin URL). No worker was killed. No Temporal server was started. The local Wekan board was not modified. G3–G6 remain open.
