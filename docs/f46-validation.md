# F46 verification — 13 October 2026

F46 restores a disposable PostgreSQL dump into a second disposable database. [D61](decision-log.md#d61--f46-backup-and-restore-drill) records the boundary. The scope is in [backup.md](backup.md).

The server was PostgreSQL 16.15 started for tests. It was not the retained product database and not the CI PostgreSQL 18.6. The database name had the disposable prefix `forge_http_`.

`TestDisposableDumpRestoresTheWorkload` inserted workload `backup-drill` and read the same id, name, and description from the restored database. One row was present. The dump measured 61.758744 milliseconds. The restore through that read measured 71.794097 milliseconds.

`TestScopeIsNotAnOffsiteEncryptedBackup` keeps `Encrypted` and `Offsite` false. Temporal history, in-process evidence, and in-process audit events are listed as unrecoverable from the dump.

Checked with Go 1.27.1:

```text
go test -count=1 -v -timeout 120s ./internal/backup
```
