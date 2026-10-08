# F12 — Durable command reconciliation

Accepted on 8 October 2026: adapt the original Kubernetes controller ticket to start/cancel reconciliation for the native API/workflow release. [D25](decision-log.md#d25--f12-durable-command-reconciliation) records the approved scope and operational defaults. This implementation supplies a Go reconciliation library, PostgreSQL leases/retries and a verified backend interface. F22 supplies the real Temporal adapter and starts a dispatcher process; no fake backend or dispatcher service is installed now.

## Components and permissions

| Component | Responsibility |
|---|---|
| Owner-facing repository/API | Validate/authenticate ownership; atomically accept run/start; insert one cancellation intent. The API can insert only command ID/run ID/kind, allowing delivery fields to use safe defaults. |
| [Store Dispatcher](../internal/store/dispatch.go) | Privileged, separate connection pool; claim eligible commands, fence acknowledgement/failure and persist retry scheduling. |
| [Reconciler](../internal/reconcile/reconcile.go) | Validate immutable run/spec envelope, invoke one bounded delivery, verify its receipt, and acknowledge or park/retry the intent. |
| Backend interface | Later Temporal adapter: verify exact owner/run/workflow/kind, reconcile ambiguous acceptance without a duplicate logical start, honor context cancellation and classify retryable errors. |
| Temporal | Authoritative execution state/history; command delivery metadata is not execution status. |

`forge_dispatcher` has SELECT on product/source records and column-limited UPDATE on delivery metadata. It cannot insert/delete commands, mutate workloads/versions/runs/ownership, create schema or access the migration ledger. It is a trusted background role that reads all owners, not a developer identity. The API's `forge_runtime` retains owner-facing permissions and no command UPDATE; its earlier table-wide command INSERT grant is revoked during upgrade. A compromised privileged process/host is outside application owner isolation.

New manual installations use [bootstrap.sql](../deploy/postgres/bootstrap.sql). Existing manually managed installations create the role once with [bootstrap-dispatcher.sql](../deploy/postgres/bootstrap-dispatcher.sql), set its password privately with psql's password command, apply `forge-migrate`, then apply [runtime grants](../deploy/postgres/runtime-grants.sql) and [dispatcher grants](../deploy/postgres/dispatcher-grants.sql) as the migrator. Never edit applied migration 0001; [0002](../internal/store/migrations/0002_command_reconciliation.sql) adds delivery metadata transactionally.

The native startup performs this retained-state upgrade automatically: append a private dispatcher secret without rotating existing passwords/cursor key, create the role, migrate and apply both grants before API startup. Secret replacement uses a private temporary file, file fsync and atomic rename to preserve old values if replacement fails. No dispatcher credential is injected into the API environment. No dispatcher unit/command, Temporal installation or container is added.

## Claim, deliver, acknowledge

1. A short PostgreSQL statement selects one eligible pending command with `FOR UPDATE SKIP LOCKED`, commits a random UUID lease token, increments the durable attempt count and returns its joined immutable run/spec/owner envelope. No row lock, transaction or acquired connection is held during backend delivery. [PostgreSQL queue-style locking](https://www.postgresql.org/docs/18/sql-select.html).
2. Start commands are eligible when due and unleased/expired. Cancel commands additionally require their run's start command to be delivered. Blocked commands are excluded. Database time governs leases and retry eligibility.
3. Validate schema, resource IDs, exact input/version association, semantic spec digest and document subset before calling the backend. The fixed research spec still rejects arbitrary tools/deployment fields. Frozen scope is not proof of current corpus approval; activities must apply trusted corpus/tool policy again.
4. Invoke the backend under a deadline. Its receipt must match the original owner, Forge run ID, `forge-run/<run UUID>` workflow ID and command kind. Copies of JSON and pointer inputs protect acknowledgement identity from adapter mutation. A receipt means verified delivery acceptance, not completion.
5. Update delivery metadata only when the command is pending, its original lease token still matches and the lease has not expired. A stale/expired token returns `ErrLeaseLost` and cannot acknowledge or reschedule a newer claim. SQL also protects command identity, prevents updates after delivery and requires acknowledged start before cancel acknowledgement.

The default loop handles one command at a time, then waits one second; claim, delivery and acknowledgement/failure each have a five-second operation deadline. Leases last 30 seconds. Configured bounds reject leases shorter than three operation timeouts; the backend must actually honor its context. Slow/noncooperative adapters can outlive leases and repeat external requests; the token fences PostgreSQL writes, not external effects. Adapter idempotency is essential. No public response/telemetry serializes this privileged envelope.

## Retry, parking and interruption

The limit is eight durable claims, including claims interrupted before a backend call. Reviewed retryable failures and expired delivery deadlines use persisted backoff of 1, 2, 4, 8, 16, 32, then 60 seconds. Attempt eight is parked. A crashed eighth claim is parked once its lease expires; it cannot produce a ninth automatic attempt. Database/fencing errors stop the loop and return safe repository errors to its future supervisor.

Unclassified backend failures, invalid payloads and receipt identity mismatches park immediately. Persist only `retryable`, `deadline`, `permanent`, `invalid_payload`, `identity_mismatch` or `attempts_exhausted`; never error text, credentials, prompts or provider responses. Parking retains `state: pending` with `blocked: true`: it is delivery investigation work, not an invented workflow failure. A blocked start leaves cancellation requested and undelivered.

An operator with privileged SQL access can inspect pending/blocked commands and retry metadata. There is no automatic unpark, reset, deletion or public operator mutation endpoint in F12. Investigate the original identity and backend acceptance before any manual intervention; later recovery tooling requires an explicit design. Ordinary stop or database restart does not delete commands, attempt counts or retry times.

Root cancellation leaves an unfinished lease to expire instead of acknowledging uncertainty. After a lost backend response or crash before acknowledgement, a new claimant uses the same original identity. The test backend models a persistent identity lookup and proves one logical start despite repeated delivery calls; this is fault injection, not a running Temporal service. F22 must verify this with real Temporal, including conflict/reuse settings and retention-limited history lookup. A missing historical execution must not be blindly started again.

Cancel acknowledgement still does not prove the workflow stopped. Completion may win after an accepted cancel request, as [F11](workload-lifecycle.md) documents. The future adapter must decide whether a terminal/not-found response proves request handling or warrants investigation; F12 does not treat a generic error as delivered. No second workflow state machine, status cache or exactly-once external-effect guarantee is introduced.

## Verification and deferred requirements

See [F12 verification](f12-validation.md) for real PostgreSQL concurrency, row-lock skipping, lease fencing, retry exhaustion, cancellation ordering, privilege checks and post-acceptance acknowledgement-loss recovery. Unit tests cover malformed envelopes, adapter mutation, cancellation/deadlines and shutdown. Native tests cover retained secrets, repeated upgrade and failed atomic replacement.

The original CRD validation, controller watches, Kubernetes ownership references, conditions and environment provisioning remain deferred. Public run submission/status/cancel/rerun endpoints, Temporal execution, recovery tooling, detailed delivery observability and worker deployment follow their own tickets. No registration load/startup performance claim follows from these correctness tests; existing Balanced measurements are unchanged.
