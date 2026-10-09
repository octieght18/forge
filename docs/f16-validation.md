# F16 verification — 9 October 2026

F16 adds authenticated asynchronous provisioning operations. [D30](decision-log.md#d30--f16-asynchronous-provisioning-operations) records the boundary. The original ticket's public namespace API is not added: developers receive an accepted operation and a status URL, and operators still apply the environment object.

Checked with Go 1.27.1. Windows ran the packages without a database, so PostgreSQL tests skipped. A temporary PostgreSQL 18 on Ubuntu then ran the database tests with `FORGE_REQUIRE_DB_TESTS=1`:

```text
go test -count=1 ./internal/provision ./internal/cli ./internal/environment
go test -count=1 ./internal/httpapi ./internal/store
```

`internal/provision` covers conflicting versions, a second open operation, cancel, a finished operation that does not change, ownership and identity failures, deletion only after `Deleting`, and timeout versus a `Ready` observation at the deadline. `internal/cli` covers `provision`, `operation status`, and `operation cancel`, including the status location. `internal/environment` covers the controller reporting `Ready` and `Failed` phases. `TestProvisioningOperations` accepted an operation, rejected a second open operation and a stale version, recorded an ownership failure, canceled through `Deleting` then `Absent`, and timed out a past deadline. Store tests covered the forward migration, a failed later migration rolling back, and the runtime role being unable to delete operation rows.

The read-only `forge_environment` role is unchanged. A kind controller therefore still does not persist operation observations. Runtime grants allow the API role to insert and update operation rows, not to delete them.

Documentation checks reported 45 Markdown documents and 332 relative links. The PowerShell checker parsed 7 scripts and 28 documented PowerShell blocks.
