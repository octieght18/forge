# F04 contract validation

Date: 5 October 2026

**Status: owner contract choices accepted; local checks passed.** Decisions D16/D17 record the detailed profile and the API shape/scope/pagination/OIDC conventions. The published CI run is linked in the F04 card.

| Acceptance criterion | Deliverable |
| --- | --- |
| Versioned workloads, versions and executions; errors/pagination | OpenAPI 3.1.1 with 19 operations (including existing GET/HEAD health), 33 schemas, workload metadata concurrency, immutable versions, logical runs, status uncertainty, cancellation/rerun, cursor pages and the F03 error envelope. Public name `/api/v1/runs` is accepted. |
| Tools, permissions, resources, deployment and desired state | Explicit research-only tool/scope/limit/model/prompt profile; platform approval required. Owner-approved amendment: unsupported infrastructure/deployment/arbitrary tool fields are rejected, consistent with the smaller release. |
| Identity, idempotency, auth placeholder limitations | Real OIDC access-token requirement, `(iss,sub)` ownership, developer/operator action matrix, object privacy and nested checks; durable owner/key idempotency contract and fingerprint helpers. No fake authentication or product route implementation. |

Checks run:

- Validate the document against an unmodified vendored official OpenAPI meta-schema, including its provenance hash and license; compile every payload schema and resolve internal references offline.
- Validate nine published examples; reject owner spoofing, malformed/duplicate-key JSON, unknown fields, unsafe paths/tools/permissions, unapproved model/workflow, missing/invalid IDs and boundary violations.
- Verify property/source order and equivalent integer spellings preserve fingerprints; changed inputs and rerun operations differ.
- Reject unavailable/current-success status, invalid observation timestamps, uncited/empty answered reports and unexplained insufficient-evidence outcomes.
- Bound parser depth/numeric exponents and reject invalid UTF-8/unpaired surrogate escapes, without leaking input values.
- `go vet` and contract tests passed on Windows. `govulncheck v1.8.0` reports no vulnerabilities after pinning patched `golang.org/x/text v0.39.0`; validator is pinned to `jsonschema v6.0.3`.

Repository-wide Windows formatting/module integrity/vet/build checks and all tests passed. Existing HTTP health/error/panic responses validate against the published schemas; product paths still return 404. CI runs the same checks on Windows/Linux plus Linux race detection and process shutdown smoke. No database/idempotency transaction, OIDC enforcement, permission lookup, immutable-version persistence, Temporal command, report hash/reference/claim support or pagination storage implementation is claimed. Those obligations remain application/integration work in later tickets.
