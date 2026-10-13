# F37 verification — 13 October 2026

F37 defines retention and a demo cleanup that cannot remove the retained database or secrets. [D52](decision-log.md#d52--f37-secrets-and-data-retention) records the boundary. The rules are in [retention.md](retention.md).

Executions stay in PostgreSQL and Temporal until an operator procedure outside this job. Evidence and audit stay in process memory. Attachments are not accepted. Secret names are environment variables, Kubernetes Secret objects, or local files. Git is not one of those places.

The cleanup command requires `--confirm delete-synthetic-demo-records`. It removes only regular files named `demo-synthetic-*` in the given directory. It refuses a `forge-native` or `forge-kubernetes` path, a directory containing `PG_VERSION`, a directory containing `secrets.json`, a secret-shaped file name, and a symlink. A refused run leaves every file in place. The command does not open a database.

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/retention ./cmd/forge-demo-cleanup
python3 scripts/check-docs.py
```

No retained product database was opened or deleted. No Kubernetes Secret was created.
