# Backup and restore — F46

[D61](decision-log.md#d61--f46-backup-and-restore-drill) records the boundary. The drill dumps one disposable PostgreSQL database and restores it into a second disposable database. It does not open the retained product database.

## Scope

The dump includes the PostgreSQL schema and rows in that database, including the workload and principal created for the drill. It does not include Temporal workflow history, in-process evidence, or in-process audit events. Those are unrecoverable from this backup. Kubernetes secrets and local secret files are not in the dump either.

The dump file is plaintext SQL in a temporary directory, mode 0600, and the test deletes it. It is not encrypted. It was not copied offsite. No offsite encrypted backup is claimed.

## Dependencies

Restore needs `pg_dump`, `psql`, a second empty database on the same server, and the dump file. The client major version has to match the server: the GitHub runner ships PostgreSQL 16, which cannot dump the CI PostgreSQL 18.6 service, so that job installs the 18.6 client and puts it first on `PATH`. It does not need Temporal, a model key, or the retained state directory.

## Drill

The source and the target were disposable databases created for the test. One workload, `backup-drill`, was inserted and then read back from the second database with the same id, name, and description.

The dump took 61.758744 milliseconds. The restore, through the query that checked the row, took 71.794097 milliseconds. That dump duration is the capture window for this sample: a write after the dump started is not in the file. There is no continuous archive. The restore duration is the recovery time for this sample, not a production objective.
