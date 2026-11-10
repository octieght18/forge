# Runbook — data restore

## Symptoms

- Product PostgreSQL rows are missing, corrupted, or need to be copied into a separate empty database for recovery drill.
- Operators need to know what a dump can and cannot recover.

## Immediate checks

1. Confirm the target is a **disposable or explicitly designated restore database**, not an accidental overwrite of the retained product database ([backup](../backup.md)).
2. Confirm `pg_dump` / `psql` major version matches the server (CI installs PostgreSQL 18.6 client when the service is 18.6).
3. List what is in scope: schema and rows in that PostgreSQL database only.
4. List what is **unrecoverable from this dump**: Temporal workflow history, in-process evidence, in-process audit events, Kubernetes secrets, and local secret files.

## Recovery

1. Take a plaintext SQL dump of the source database (mode 0600 on the file). [F46](../f46-validation.md) measured dump **61.758744 ms** and checked restore **71.794097 ms** on disposable PostgreSQL 16.15 for one workload row (`backup-drill`).
2. Restore into a second empty database on the same server.
3. Verify id, name, and description (or the relevant product keys) match.
4. Re-establish Temporal and worker processes separately. History is not in the dump; runs may need documented re-execution rather than silent resume if history is gone.
5. Do not claim encryption or offsite copies unless those controls exist. F46 keeps `Encrypted` and `Offsite` false.

## Related rollback (not the same as restore)

For a bad canary pin, use [rollback](../rollback.md), not this dump path. [F47 bad_rollout](../failure-injection.md) restored the staging pin in **33 µs** without a second write when no regression remained. [F45](../f45-validation.md) measured detection 292 ns and pin write 52.762 µs. Automatic restore stops after an applied forward-only migration, a completed model call, or a missing previous pin.

## Corrective actions from F47

- **database_outage:** prefer role/login recovery when the store is temporarily unavailable (17329 µs disposable-role sample) before leaping to dump restore.
- **bad_rollout:** prefer pin restore for delivery regressions; dump restore does not reverse a migration or unsend a model call.

## Limits

No offsite encrypted backup was created. The retained product database was not opened in F46 or F47. This runbook is a drill procedure for disposable databases unless an operator explicitly designates another target.
