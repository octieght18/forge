# Runbook — authentication and authorization failures

## Symptoms

- Product routes return `401` (missing/invalid token) or `403` (service token or forbidden role action).
- Owner A receives `404` for owner B's ids (expected) or cannot read their own records (unexpected).
- Cross-tenant ids return `404` (expected for another tenant).

## Immediate checks

1. Confirm the caller uses a Keycloak user access token with role `developer` or `operator`, not a service token. Service tokens stay `403` on product routes ([identity](../identity.md)).
2. Confirm issuer, audience, signature, and expiry. Access-token lifetime is five minutes for the login helper path.
3. For mutations, confirm the subject owns the record. Operators may read across owners inside the same tenant/project and must not mutate another owner's workload, version, run, or cancel ([authorization](../authorization.md)).
4. Distinguish **other-owner 404** and **other-tenant 404** from outages. [F47 database_outage](../failure-injection.md) recovered owner reads while the other subject stayed at 404. [F47 quota_exhaustion](../failure-injection.md) kept the other subject unable to read the first workload.

## Recovery

1. Re-run `forge-login` / `scripts/local-stack.ps1 login` to refresh the private token file. Do not paste tokens into tickets or commits.
2. If JWKS or discovery is slow or canceled, rely on existing F10 cancellation behavior: caller cancel must not leave a poisoned shared key fetch; a subsequent caller can succeed.
3. If registration returns `401` after a canary event, re-check the API path itself. [F45](../rollback.md) re-checked that a missing token still returned 401 after pin restore; that check used the existing API, not a live canary proxy.
4. Denied tools must not start a process. A `tool_denied` failure is not retryable.

## Corrective actions from F47 and earlier

- After database login loss, verify isolation still holds (other subject 404) before declaring recovery complete.
- After quota denial, do not treat 404 on another subject as data loss.
- Redact secret-shaped tool errors in evidence ([F47 tool_model_outage](../failure-injection.md)).

## Limits

G3 and G4 remain open. This runbook does not claim organization-level isolation, token revocation before expiry, mesh workload identity, or an issued TLS client path. Kind does not enforce NetworkPolicy in this repository.
