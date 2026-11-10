# Simulated incident reports — F48

These reports are **simulated**. They were written from measured failure-injection and backup/rollback samples in this repository. No production customer outage occurred. No live worker process was killed and no Temporal server was stopped for these narratives.

| ID | Title | Primary F47 / ops evidence |
|---|---|---|
| [INC-SIM-001](INC-SIM-001-database-login-loss.md) | Database login loss during owner read | `database_outage`, F46 backup |
| [INC-SIM-002](INC-SIM-002-worker-and-tool-outage.md) | Worker resume and tool/model outage | `worker_loss`, `tool_model_outage`, F29 |
| [INC-SIM-003](INC-SIM-003-bad-rollout-and-quota.md) | Bad canary pin and quota exhaustion | `bad_rollout`, `quota_exhaustion`, F45 |

Runbooks: [docs/runbooks](../runbooks/README.md).
