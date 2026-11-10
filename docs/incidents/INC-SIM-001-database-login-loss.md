# INC-SIM-001 — Database login loss during owner read (simulated)

**Label:** simulated incident report  
**Date of narrative:** 10 November 2026  
**Evidence basis:** [F47 `database_outage`](../validation/f47-failure-injection.json), [F46 backup drill](../f46-validation.md), [data-restore runbook](../runbooks/data-restore.md)

## Timeline

| Step | Observation |
|---|---|
| T0 | Disposable PostgreSQL role loses login; existing sessions terminated (injection). |
| T0+ | `/healthz` remains HTTP 200. |
| T1 | Owner workload read returns 503 while the store is unavailable. |
| T2 | Role login restored. Owner read returns 200. Other subject still receives 404. |
| T3 | Row count remains one. Duration of the measured scenario: **17329 µs**. |

## Cause

Injected loss of login for the disposable test role. The retained product database was not stopped. Liveness stayed up while product reads failed, which can look like an application bug if health alone is trusted.

## Recovery

1. Follow [auth-failures](../runbooks/auth-failures.md) isolation checks: other-subject 404 must still hold after recovery.
2. Prefer role/credential restoration over dump restore when the failure is login loss ([data-restore](../runbooks/data-restore.md)).
3. If rows are actually lost, use the F46 disposable dump/restore path (dump **61.758744 ms**, checked restore **71.794097 ms**) into a separate empty database. Temporal history and in-process evidence remain unrecoverable from that dump.

## Follow-up

- Document that `/healthz` is not product-store readiness (already true in F47 limits).
- Keep G3/G4 open; disposable-role injection is not a production multi-AZ failover test.
- Corrective action tied to F47: verify isolation after store recovery before closing the incident.
