# INC-SIM-003 — Bad canary pin and quota exhaustion (simulated)

**Label:** simulated incident report  
**Date of narrative:** 10 November 2026  
**Evidence basis:** [F47 `bad_rollout`](../validation/f47-failure-injection.json), [F47 `quota_exhaustion`](../validation/f47-failure-injection.json), [F45 rollback](../rollback.md), [provisioning-stuck runbook](../runbooks/provisioning-stuck.md)

## Timeline

| Step | Observation |
|---|---|
| T0 | Canary window shows 40 errors in 100 requests and 800 ms latency (injection used by F45/F47). |
| T1 | F45 detection **292 ns**; pin write **52.762 µs**; whole call **54.421 µs**. Staging pin returns to `local-overlay`. Execution ids retained. |
| T2 | F47 `bad_rollout` restore **33 µs**, one pin write; second call with no regression does not write again. |
| T3 | Separately, tenant workload cap set to one (`quota_exhaustion`). Second create limited. Duration **16572 µs**. One row retained. Other subject cannot read the first workload. |

## Cause

Regressed canary pin plus an intentional per-process workload cap. Automatic pin restore does not reverse migrations, unsend model calls, rewrite Temporal history, or change a live cluster. Quota is not distributed and does not alter `small-v1`.

## Recovery

1. Run canary rollback when error rate > 1% or latency exceeds the window maximum ([rollback](../rollback.md)).
2. Re-check unauthenticated registration still returns 401 after pin restore (F45 check on disposable DB).
3. For quota denials, inspect tenant workload/pending-run counts before raising limits ([provisioning-stuck](../runbooks/provisioning-stuck.md)).
4. Human gates: applied forward-only migration, completed model call, or missing previous pin.

## Follow-up

- Keep cloud promotion refused when the environment was not created ([promotion](../promotion.md)).
- Corrective action tied to F47: idempotent pin restore (no second write) and single-row quota enforcement under the test cap.
- Do not close G5; live Argo/DNS/TLS and applied EKS remain undemonstrated.
