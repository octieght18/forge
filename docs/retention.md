# Secrets and retention — F37

[D52](decision-log.md#d52--f37-secrets-and-data-retention) records the boundary. Credentials and retained evidence are platform concerns. This page states where they live and what the demo cleanup is allowed to remove.

## Retention and deletion

| Class | Where it lives | How long | Deletion |
|---|---|---|---|
| Executions | PostgreSQL runs and start/cancel commands. Temporal history for `forge-run/<run UUID>` | For the life of the product database and that workflow history. No expiry clock is set | Principals, versions, and runs reject DELETE. The demo cleanup does not connect to PostgreSQL or Temporal |
| Evidence | Worker process memory, retention `workflow_history` or `compensated` | Until process exit, or until compensation changes the label | There is no product evidence table. The demo cleanup does not clear worker memory |
| Audit | API and worker process | Until process exit | Events are not a database table. They do not store prompts or tokens |
| Attachments | None | The API does not accept attachments | Nothing is stored |

Ordinary native and kind stop still retain the product database, identity data, and local secret files. The native `purge` action is a separate confirmed operator action on the state directory. It is not this cleanup.

A backup of the product database is an operator copy of those retained rows. Deleting a synthetic scratch file does not restore a backup and does not remove one. A backup stored under the retained state directory is out of scope for this job.

## Where secrets live

| Name | Place |
|---|---|
| `FORGE_DATABASE_URL`, `FORGE_MIGRATION_DATABASE_URL`, `FORGE_MODEL_API_KEY` | Process environment |
| `postgres-admin`, `keycloak-db`, `api-db`, `migration-db`, `cursor-key` | Kubernetes Secret objects, referenced with `secretKeyRef` |
| `secrets.json`, `cursor.key`, token files, kubeconfig | Local files under the retained state directory, mode 0600 |

Those values stay out of Git. `.gitignore` excludes `.env`, `.env.*`, `*.pem`, and `*.key`. Specs, logs, and audit events do not contain connection strings or tokens. The Kubernetes Deployment passes database credentials by secret reference, not as a literal environment value.

## Demo cleanup

`forge-demo-cleanup` deletes regular files in one explicit directory when every condition holds:

- `--confirm` is `delete-synthetic-demo-records`
- the directory is not inside `forge-native` or `forge-kubernetes`
- the directory has no `PG_VERSION`, `secrets.json`, `cursor.key`, kubeconfig, `.env`, or other secret-shaped name
- the file name starts with `demo-synthetic-`
- the file is a regular file in that directory, not a link or a directory

Any other request is refused and nothing is removed. The command does not take a database URL. It does not delete product rows, Temporal history, or secret files.

```sh
go run ./cmd/forge-demo-cleanup --dir /tmp/forge-demo-scratch --confirm delete-synthetic-demo-records
```
