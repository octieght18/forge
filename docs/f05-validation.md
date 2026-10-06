# F05 validation evidence

Owner decisions accepted on 6 October 2026: pgx/explicit SQL and JSONB; explicit checksummed forward migrations; workloads, versions, runs and durable start/cancel intents; repository owner checks, separate SQL roles, no RLS initially, and real PostgreSQL tests. These follow F04 and preserve the smaller first release.

Implemented in [store](../internal/store/store.go), [version repository](../internal/store/versions.go), [run/command repository](../internal/store/runs.go), [migration runner](../internal/store/migrate.go), [initial SQL migration](../internal/store/migrations/0001_product_state.sql) and [operator guide](persistence.md).

Verification covers concurrent owner/name creation; optimistic update conflicts; unique version numbering; same-key submission/cancel races; semantic idempotency; cross-owner/direct/nested/collection access and operator mutation denial; immutable rows; exact OIDC identities and maximum lengths; malformed input; run/start rollback; rerun conflict/dependency retry behavior; stable keyset pagination; connection recreation; concurrent/checksummed/failed migrations; and restricted runtime privileges.

The local PostgreSQL 18.6 restart harness seeds durable records, restarts the server and verifies the same workload/version/run/key/Workflow ID plus pending start/cancel commands from another Go process. GitHub CI repeats this against a digest-pinned PostgreSQL service and requires database tests to run.

Local results: module verification, formatting, vet and build passed; the Windows suite against real PostgreSQL passed 25 top-level tests and 46 named subtests with zero failures. The Linux suite passed with race detection, followed by successful seed/server-restart/verify phases. Govulncheck v1.8.0 reported no vulnerabilities. All 17 Markdown documents and 61 local links were checked. The restart test is deliberately skipped in the ordinary suite and invoked separately by its harness.

Published commit and CI evidence are recorded on the F05 Wekan card after CI completes. Product HTTP endpoints, actual OIDC token validation, source approval, Temporal delivery/status, signed cursors, artifact storage, live models, retention and backup/restore remain unimplemented. Passing repository tests must not be presented as end-to-end authentication or workflow execution.
