# Canary rollback — F45

[D60](decision-log.md#d60--f45-canary-rollback) records the boundary. A regression in the canary window restores the previous staging pin. The function does not delete execution rows.

## Detection

`NeedsRollback` is true when the error rate is above 1 percent or the observed latency is above the maximum supplied for that window. The test injected 40 errors in 100 requests and a latency of 800 milliseconds against a 200 millisecond maximum.

On this machine that detection step measured 292 nanoseconds. Writing the previous pin measured 52.762 microseconds. The whole call measured 54.421 microseconds. Those are one sample from `TestRollbackRestoresPinAndKeepsExecutions`, not an SLO.

## Restored pin and retained rows

The staging pin returned to `local-overlay`. The execution identifiers passed into the call were copied out unchanged. Changing the caller's slice afterward did not change the result. No product database was opened and no row was deleted.

The registration API was checked again on the disposable database. A request without a token still returned 401. That is the existing API, not a live canary proxy.

## Human intervention

Automatic restore does not run when a forward-only migration has already been applied, when a model call has already completed, or when the previous pin is missing. Temporal history is not rewritten in any of these cases. An operator investigates those gates before choosing another pin.
