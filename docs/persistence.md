# F05 — PostgreSQL product persistence

The owner accepted pgx v5 with explicit SQL, validated JSONB specs/inputs, explicit transactional forward migrations, separate migration/runtime roles, application owner checks without RLS, and real PostgreSQL tests on 6 October 2026. F05 implements workloads, immutable versions, runs and durable start/cancel intents. HTTP product routes, OIDC enforcement, Temporal dispatch and evidence/report storage follow their implementation tickets.

## Storage and access

| Table | Responsibility and invariant |
| --- | --- |
| `forge.principals` | Immutable OIDC issuer/subject pairs. Separate internal UUIDs reference ownership. SHA-256 columns verified by CHECK constraints provide bounded indexes for maximum-length identities; lookup also compares exact original strings and fails closed on a digest collision. |
| `forge.workloads` | Owner-scoped unique names, metadata, positive revision and timestamps. Updates lock the owned row, compare the expected revision, then increment it. Ownership, creation timestamp and identity cannot change. |
| `forge.versions` | Validated research spec JSONB, semantic spec digest and unique positive number within a workload. Registration locks that workload before allocating the next number. Database triggers reject updates/deletes. Identical declarations may create different numbered versions, as F04 permits. |
| `forge.runs` | Frozen version/input, stable `forge-run/<UUID>` Workflow ID, lifetime owner/key uniqueness and operation fingerprint. Composite foreign keys enforce workload/version/owner/rerun associations. Runs are immutable; this table does not implement a second execution state machine. |
| `forge.commands` | One start and at most one cancel intent per run, initially pending. Start is inserted in the same transaction as its run. Delivery state fields are reserved for the later dispatcher; runtime has no update grant yet. |
| `forge.schema_migrations` | Embedded SQL filenames, SHA-256 checksums and application timestamps. Only migration credentials access this ledger. |

The repository requires a **trusted principal** carrying validated issuer, subject and a mapped `developer` or `operator` role. Constructing this Go value is not authentication. No product route calls the repository yet. Every public read/list method filters ownership in SQL; an operator can inspect all owners. Mutations require the exact issuer/subject, including for operators. Missing and inaccessible objects return the same `ErrNotFound`. Principals sharing a subject under different issuers remain distinct. Internal post-authorization transaction reads use the just-authorized/inserted ID.

The runtime database credentials are privileged infrastructure credentials, unavailable to developers. Without RLS, a direct runtime SQL connection can read all product rows; the API/repository policy is the isolation boundary. Host administrators and migration credentials can inspect or alter storage. The tests prove repository ownership behavior and SQL role separation, not OIDC validation or protection against a compromised runtime process.

Payloads use F04's strict validator before insertion. SQL constraints cover referential identity, uniqueness, base types and immutability; they do not duplicate the entire research JSON Schema or approve corpus availability. A caller must apply trusted corpus/source policy before registering a version and again in activities. Submission checks document selection is a subset of that immutable version. PostgreSQL cannot represent NUL characters in text/JSONB; rejected database encodings return a safe invalid-input error. No driver diagnostics, inputs or connection strings are exposed through repository errors.

## Transaction behavior

- Workload creation resolves the exact immutable identity and inserts the workload in one transaction. Concurrent same-owner/name attempts yield one creation and conflicts; different owners can reuse names.
- Metadata updates lock the authorized workload and compare revisions. Exactly one concurrent writer using a given revision succeeds. The service will map this to F04 ETags/precondition responses.
- Version numbering uses the same per-workload row lock without changing metadata revision. Queries use explicit schema/table names.
- Submission authorizes the workload/version before looking up the owner/key. A transaction-level advisory lock serializes that key; hash collisions only delay unrelated keys. The actual unique constraint and exact fingerprint comparison decide identity. Same semantic input returns the existing run; changed operation/input conflicts. A failed start insert rolls back both run and key.
- Controlled rerun shares the owner/key scope and copies the parent's immutable inputs. The service supplies a terminal-status check from Temporal for a new acceptance; an already accepted matching retry bypasses that dependency check. F05's callback boundary does not implement or prove Temporal status.
- Cancellation inserts one durable intent with conflict-safe retry. Commands are read in start/cancel order. This ordering is not a delivery guarantee: the later dispatcher must gate cancel delivery on acknowledged start, resolve ambiguous starts, prevent Workflow ID reuse and implement bounded retries/leases.

Transaction locks automatically release at transaction end. There is no PostgreSQL/Temporal distributed transaction, automatic SQL retry loop or exactly-once inference claim. Callers supply bounded contexts; cancellation rolls back pending writes. An uncertain commit response can be retried safely using the same run idempotency key. Workload/version creation has no idempotency guarantee. [PostgreSQL locking](https://www.postgresql.org/docs/18/explicit-locking.html), [pgx v5.11.0](https://pkg.go.dev/github.com/jackc/pgx/v5@v5.11.0).

Repository pagination uses typed internal positions: workloads/runs `(created_at DESC, id DESC)`, versions descending number. Filters apply before the limit; lists accept 1–100 entries. Fetch an extra entry when implementing the public next-page response. These positions are not public cursors. Signing, caller/collection/filter binding, expiration and cursor key management still require consultation in the endpoint ticket.

## Explicit schema evolution and roles

PostgreSQL **18.6** is the tested baseline. CI pins the `postgres:18.6-bookworm` manifest digest. Local test PostgreSQL comes from Ubuntu's package repository. A complete Compose deployment and its persistent volume/backup setup remain later work; no production database is provisioned by this ticket. [PostgreSQL supported versions](https://www.postgresql.org/support/versioning/).

For an operator-managed local PostgreSQL instance:

1. Run [bootstrap.sql](../deploy/postgres/bootstrap.sql) once as an administrator. It creates `forge`, `forge_migrator` and `forge_runtime`, without embedded passwords. It deliberately fails if these already exist; review existing roles/databases instead of deleting them.
2. Set each role's separate password using `psql`'s `\password`, and configure a private password file or inject credentials through your local secret mechanism. Never commit credentials or paste connection strings into logs. Only loopback development may use `sslmode=disable`; exposed deployments require a deliberate TLS/network design.
3. Set `FORGE_MIGRATION_DATABASE_URL` to the migration connection, run the command below, then run [runtime-grants.sql](../deploy/postgres/runtime-grants.sql) as the migrator against `forge`.
4. Future service wiring must use the runtime connection only. F05 does not add a database setting to the health-only API or change its readiness behavior.

Example in PowerShell with credentials supplied privately outside the URL:

```powershell
$env:FORGE_MIGRATION_DATABASE_URL = 'postgres://forge_migrator@127.0.0.1:5432/forge?sslmode=disable'
go run ./cmd/forge-migrate
Remove-Item Env:FORGE_MIGRATION_DATABASE_URL
```

The command applies embedded migrations with a one-minute context. A dedicated transaction-level advisory lock serializes migrators; schema changes and ledger writes commit together. Rerunning verifies checksums and does nothing for applied files. Changed, unknown or gapped history fails closed. API startup never applies migrations. Add a new ordered SQL file for a forward change; never edit an applied file. There are no destructive down migrations. Backups/restores, upgrade compatibility and retention/deletion policy remain future operator decisions.

## Verification

The normal Windows/Linux checks still run without a database; real database tests explicitly skip when no test DSN is configured. The separate CI PostgreSQL job sets `FORGE_REQUIRE_DB_TESTS=1`, which makes a missing DSN a failure, and runs all tests with race detection.

```powershell
# Supply a disposable server admin connection privately; tests create their own
# uniquely named databases/roles and require CREATE DATABASE/CREATE ROLE.
$env:FORGE_REQUIRE_DB_TESTS = '1'
go test -count=1 -timeout 120s ./...
```

Set `FORGE_TEST_ADMIN_DATABASE_URL` to the disposable server before that command. Tests never drop the supplied database. They create separate migration/runtime roles, exercise real transactions, key races, ownership, limits, SQL privileges, immutable records, migration checksums/rollback and connection-pool recreation. Synthetic corpus examples require no model call or credentials.

The restart harness separately invokes `TestPostgreSQLRestartPersistence` with `FORGE_TEST_RESTART_PHASE=seed`, writes IDs to `FORGE_RESTART_PROOF_FILE`, restarts the actual PostgreSQL service, then invokes phase `verify` in a new Go process. It checks retained workloads/versions, the same accepted run/key/Workflow ID and both pending commands. CI and the local Linux run use this sequence. A clean database restart demonstrates durable local persistence; it does not establish crash recovery objectives, backup recovery, availability or distributed dispatch behavior.
