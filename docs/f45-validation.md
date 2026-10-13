# F45 verification — 13 October 2026

F45 rolls a regressed canary pin back and leaves execution identifiers in place. [D60](decision-log.md#d60--f45-canary-rollback) records the boundary. The sample is in [rollback.md](rollback.md).

`TestRollbackRestoresPinAndKeepsExecutions` injected 40 errors out of 100 requests and an 800 millisecond latency. The previous pin `local-overlay` replaced `bad-release`. The two execution identifiers were still present. Detection was 292 nanoseconds, the pin write was 52.762 microseconds, and the call was 54.421 microseconds.

`TestRollbackStopsForHumanGates` left `bad-release` in place for a migration that already applied, a completed model call, and a missing previous pin. Each result named the human gate. No execution identifier was removed.

`TestRegistrationHTTPPostgreSQL` passed again. A missing token returned 401.

Checked with Go 1.27.1:

```text
go test -count=1 -v -run 'TestRollback' ./internal/delivery
go test -count=1 -timeout 180s ./internal/httpapi -run 'TestRegistrationHTTPPostgreSQL'
```
