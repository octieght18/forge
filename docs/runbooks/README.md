# Incident runbooks — F48

These runbooks are for operators of the demo Forge control plane. They use measured recovery behavior from [F47 failure injection](../failure-injection.md), [F46 backup](../backup.md), and [F45 rollback](../rollback.md). They are not production SLOs.

| Runbook | Symptom |
|---|---|
| [Provisioning stuck](provisioning-stuck.md) | Environment operation stays `accepted` or never reaches a terminal controller phase |
| [Execution stuck](execution-stuck.md) | Run stays non-terminal; worker or tool appears hung |
| [Auth failures](auth-failures.md) | 401/403 on product routes; unexpected 404 across owners or tenants |
| [Data restore](data-restore.md) | Need to recover PostgreSQL rows after loss or corruption |

Simulated incident reports that exercise these procedures are under [docs/incidents](../incidents/README.md).
