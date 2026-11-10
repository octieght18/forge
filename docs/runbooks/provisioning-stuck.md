# Runbook — provisioning stuck

## Symptoms

- `POST /api/v1/workloads/{id}/environments` returned `202` with status `accepted`, but the status URL never advances.
- `scripts/environment.ps1 status` shows no owned namespace, or a partial boundary that does not converge.
- Operator apply appears to hang or the controller logs identity-lookup failures.

## Immediate checks

1. Confirm the operation id and workload id from the `202` body and the private token file used to create them.
2. Read the operation status with the same developer token. Other subjects must continue to receive 404 for this workload ([F47 quota and database isolation](../failure-injection.md)).
3. Confirm the environment controller is running and its database role is the read-only product role. The controller must not write operation status ([self-service contract](../self-service-contract.md)).
4. Confirm an operator has run `scripts/environment.ps1 apply` for the submitted `ForgeEnvironment`. Developers do not receive a kubeconfig.

## Recovery

1. If identity lookup failed and was later restored, wait for the next reconcile. [F18](../f18-validation.md) measured in-process convergence at 9.4041 ms with one failure and two successes under the hang guard; that is a fake-client sample, not cluster wall time.
2. If a partial namespace exists, re-apply the same environment intent. Reconciliation must converge without duplicate owned namespaces ([F18](../f18-validation.md), [G2](../g2-validation.md)).
3. If the operation already observed `Failed`, leave it failed. Do not reopen it.
4. If the API process is rate-limiting provisioning (`429` with `Retry-After: 1`), wait and retry. The limit is per process (default 100 pending runs / 100 workloads) and is not distributed ([F35](../f35-validation.md), [F47 quota_exhaustion](../failure-injection.md) at a test cap of one: 16572 µs, one row retained).

## Corrective actions from F47

- **quota_exhaustion:** a second create under a cap of one returned the quota error and left a single row. Do not raise `small-v1` to clear a stuck provision; inspect the tenant workload count first.
- **database_outage:** when the disposable role lost login for 17329 µs, `/healthz` stayed 200 while workload reads returned 503 then recovered for the owner. Treat readiness separately from product-store availability.

## Limits

No live kind apply is required to follow this runbook's checks against HTTP fixtures. Kind NetworkPolicy is not enforced. Cloud promotion and live Argo reconciliation are out of scope when the cloud environment was not created.
