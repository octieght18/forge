# F12 verification

The owner accepted adaptation to command reconciliation and its separate role, leases, deadlines, retry cap and later Temporal deployment on 8 October 2026. See [D25](decision-log.md#d25--f12-durable-command-reconciliation) and [implementation guide](command-reconciliation.md). Original Kubernetes requirements remain deferred; the card records the amended acceptance checklist.

| Scenario | Observable assertion |
|---|---|
| Concurrent consumers and owners | Twelve concurrent claim attempts obtain each eligible start once; joined owner/workflow identity remains correct and cancel is not claimed early. |
| Locked queue row | A separately held row lock is skipped so another eligible command can be claimed. |
| Interrupted worker | An active lease cannot be reclaimed; an expired lease gets a new token and incremented attempt; old acknowledgement/failure writes are rejected. |
| Backend accepts, acknowledgement is lost | Recreate pool/reconciler, expire the lease, resolve the same workflow identity; two delivery calls create one logical start, then deliver cancel. |
| Transient error/exhaustion | Retry times remain durable and future-due; eight attempts park pending intent, including an eighth claim interrupted before acknowledgement. |
| Wrong acknowledgement/permanent error | Wrong owner/run/workflow/kind and unclassified errors park pending commands; raw backend details are not stored. |
| Malformed declaration | Invalid source subset, spec digest, owner or input/version association never reaches the backend. |
| Database roles/identity | Dispatcher cannot insert/delete/mutate product identity or access migration history; API cannot update delivery or insert manufactured acknowledgements. SQL rejects cancel acknowledgement before start and delivered-state reversal. |
| Deadline/shutdown | A late successful receipt is not acknowledged; deadline is retryable, while root shutdown leaves its lease to expire. Idle loop exits on cancellation. |
| Credential upgrade | Old values/cursor remain; repeated upgrade writes nothing; malformed configuration and failed atomic replacement preserve old data. |

Use `go test -race -count=1 -timeout 120s ./...` with the existing disposable PostgreSQL test setup and `FORGE_REQUIRE_DB_TESTS=1`. Tests create private random databases/roles and clean only those fixtures. The acknowledgement-loss and expired-time tests are controlled fault injection in recreated clients, not OS process kills or real Temporal recovery. The backend is a test double with identity lookup; its behavior is not proof of the future adapter.

Local Windows formatting/module verification/vet/full tests/build passed; database-dependent Windows tests skip without a test DSN. The full Linux suite passed against real PostgreSQL with race detection: 171 test/subtest pass events across nine tested packages and zero failures. Eight native lifecycle/credential tests passed. Documentation checks passed 34 Markdown documents and 246 relative links; PowerShell checks passed four scripts and 16 documented blocks.

The retained native stack on this PC upgraded from its earlier schema/configuration and passed Windows API/identity loopback readiness. Digest-only before/after checks confirmed identical existing credentials/cursor and workload/version/run records, with the new dispatcher secret prepared. No live dispatcher started; private digests/test outputs remain local. The disposable PostgreSQL migration test additionally verifies preservation of accepted run/idempotency/Workflow IDs and both pending commands from migration 0001 through 0002.

Normal Windows/Linux checks, PostgreSQL race/restart and real Keycloak/native quickstart CI remain required. Exact completed checks are linked on the card after publication. There is no live dispatcher, real Temporal/MCP/model call, Kubernetes integration, public run route or performance rerun in this ticket. Raw F10 profiles stay local.
