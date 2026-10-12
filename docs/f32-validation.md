# F32 verification — 12 October 2026

F32 stores a tenant and a project on workloads, versions, and runs. [D47](decision-log.md#d47--f32-tenant-and-project-boundaries) records the boundary.

Existing rows are not deleted. Migration `0004_tenant_project.sql` adds the demo tenant `00000000-0000-4000-8000-000000000001` and demo project `00000000-0000-4000-8000-000000000002` as column defaults, so issuer/subject rows receive that boundary without an update of immutable records. New writes copy the caller's tenant and project. An empty caller uses the demo pair.

Checked with Go 1.27.1 and a disposable PostgreSQL database:

```text
go test -count=1 ./internal/store ./internal/httpapi ./internal/environment ./internal/reconcile
```

`TestCrossTenantEndpointsStayHidden` creates a workload, version, run, and provision operation in a second tenant. A developer and an operator on the demo tenant receive 404 for get workload, patch workload, get and create version, create and get operation, create run, get run, cancel, history, result, and rerun. Operator create-operation stays 403, which is the existing role rejection and does not return the foreign record. The operator workload list does not contain the other tenant's name.

`TestPostgreSQLCommandMigrationRetainsAcceptedRuns` seeds a pre-tenant run and still reads it after the migration. The idempotency key and both commands remain.

Evidence has no product table. A cross-tenant caller cannot read the run, so the API does not reach that run's evidence. This is not organization-level isolation. G3 stays open.
