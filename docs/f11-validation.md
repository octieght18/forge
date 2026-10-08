# F11 verification

The owner accepted adaptation of F11 to the API/workflow release and the three lifecycle consistency safeguards on 8 October 2026. See [D24](decision-log.md#d24--f11-research-specification-and-lifecycle) and [specification/lifecycle guide](workload-lifecycle.md). The original provisioning acceptance items are explicitly deferred, with the revised checklist recorded on the project card.

The shared OpenAPI schema rejects delivered cancellation before start acknowledgement, dispatched/not-started contradictions, and partial cached observations. Tests cover all six Temporal outcomes across submission/cancellation combinations, valid pending-start observations and cancellation/completion races, all cached outcome/timestamp pairs, and enforcement inside a run page. Existing tests continue to compile the official OpenAPI meta-schema, resolve references, validate examples and reject unsupported research declarations.

Local Windows formatting, module verification, vet, full tests and build passed. PostgreSQL-dependent tests skip in that local run without a test DSN. Linux contract tests passed with race detection; the documentation checker passed 32 Markdown documents and 227 relative links; PowerShell validation passed four scripts and 16 documented blocks.

Reproduce with `go test -count=1 ./internal/contract`, then the normal formatting/module/vet/test/build and documentation checks. The hosted pipeline also runs Linux race tests against real PostgreSQL, retained-run restart checks and the real Keycloak/native quickstart. Exact completed validation is linked on the card after publication.

This is executable contract validation, not a tested Temporal dispatcher. Schema validation cannot prove an observation's source/freshness or a command's delivery. Run HTTP routes still return 404; no status cache, controller, worker or external model integration is added. Existing persistence/ownership tests cover the unchanged repository. No performance rerun is needed because request handling and registration schemas are unchanged; Balanced targets and historical observations remain intact. Raw F10 profiles stay local.
