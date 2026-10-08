# F04 — Workload API contract

**Status: accepted contract choices, recorded on 5 October 2026.** F04 supplies the versioned contract, executable validation and examples; F05 adds persistence and F06 implements the seven workload/version registration operations with real OIDC/owner checks. F07 starts the complete native local stack. Run/history/evidence/report operations below remain future contracts. Use the [current API reference](api-examples.md) and [quickstart](quickstart.md) for availability and runnable requests. The default `go run ./cmd/api` command alone still serves health-only mode.

## Resource model

| Resource | Behavior |
| --- | --- |
| Workload | Owner-private metadata, lowercase unique name per owner, server-issued UUID, revision/ETag and timestamps. Register/read/list; PATCH metadata with `If-Match`. No delete or ownership transfer. |
| Version | Immutable, explicit research spec and content fingerprint; UUID and monotonically increasing per-workload number. Register/read/list; no update/delete. |
| Run | Forge research job ID independent of Temporal IDs; exact version, question and source selection frozen. Durable acceptance, reads/list, cancel and controlled rerun. |
| History | Sanitized Temporal events, ordered by sequence; unavailable Temporal returns 503 rather than fabricated history. |
| Evidence/report | Persisted source passages, hashes and structured claims/citations; references must resolve to the same run and snapshot. |

The accepted public name is `/api/v1/runs`, matching the architecture's logical run identity. The original ticket's “executions” terminology refers to these jobs; it is not a second resource or alias. The [OpenAPI document](../internal/contract/openapi.json) is authoritative for payloads. JSON is chosen so the Go validator embeds the same document without a YAML translation or duplicated schema definitions.

## Owner consultation

Accepted by the owner on 5 October 2026:

1. Use workloads, immutable versions and runs; publish contract/validation now, with runtime endpoints later.
2. Amend the original broad declaration criterion to the smaller API/workflow release: fixed corpus tools and source scope supported; arbitrary tools, write/network permission requests, CPU/memory requests, deployment policy and desired-state control rejected explicitly.
3. Cursor pagination default 20/max 100; existing error envelope; required run idempotency keys; actual OIDC contract with no fake auth.
4. Exact identifiers, metadata concurrency and schema ceilings below; structured report claims plus limitations.

The detailed profile and the remaining combined API/scope conventions were explicitly accepted in two consultation replies.

## Explicit version profile and limits

| Item | Accepted convention |
| --- | --- |
| IDs | Server-issued lowercase UUID v4 for workloads, versions, runs and evidence; never client ownership fields. |
| Mutation body | At most 64 KiB, valid UTF-8 JSON, one document, no duplicate keys or unknown fields. |
| Metadata | Name 1–63 lowercase slug characters, description at most 1,024 characters. PATCH requires a strong `If-Match` ETag; 428 when missing, 412 when stale. |
| Workflow | Exactly `research.v1`; exactly `corpus.search` and `corpus.read`; `research-report.v1` prompt contract. |
| Snapshot/source scope | Exact SHA-256 snapshot identifier; 1–64 unique document IDs. Platform policy must approve availability/permissions, then each run chooses an explicit subset. IDs are not paths or URLs. |
| Retrieval/output bounds | Version requests 1–16 passages and 128–2,048 output tokens. The example explicitly chooses 8 passages/1,024 tokens. These are bounds, not F08 throughput or quality targets. |
| Question | 1–4,000 Unicode characters, including at least one non-whitespace character. Preserve text exactly for fingerprints. |
| Model | Selected free OpenRouter model only. Routing, credentials, no paid fallback and data policy are operator-controlled, not workload fields. |
| Report | Up to 32 structured claims, each with 1–16 evidence IDs; up to 16 limitations. Answered reports need claims; insufficient/conflicting outcomes need limitations. |

All spec fields are explicit; registration has no silent schema defaults. Scope/tool arrays are sets, with unique elements. The parser also caps nesting at 64 levels and numeric spellings at 128 characters with exponents from -308 to 308, and rejects unpaired UTF-16 escapes. These guard validation resources and preserve unambiguous strings; ordinary integer spellings remain equivalent. Source order, JSON property order and equivalent integer spellings do not change the semantic fingerprint. Changing the question/version/selection does. The request fingerprint is a comparison value, not authorization or durable idempotency by itself.

## Paths and response conventions

| Operations | Path |
| --- | --- |
| GET / POST | `/api/v1/workloads` |
| GET / PATCH | `/api/v1/workloads/{workload_id}` |
| GET / POST | `/api/v1/workloads/{workload_id}/versions` |
| GET | `/api/v1/workloads/{workload_id}/versions/{version_id}` |
| GET / POST | `/api/v1/runs` |
| GET | `/api/v1/runs/{run_id}` |
| POST | `/api/v1/runs/{run_id}/cancel` |
| POST | `/api/v1/runs/{run_id}/rerun` |
| GET | `/api/v1/runs/{run_id}/history` |
| GET | `/api/v1/runs/{run_id}/evidence` |
| GET | `/api/v1/runs/{run_id}/report` |

Existing public `GET/HEAD /healthz` and `/readyz` are also documented. Workload/version product routes are available in explicitly configured product mode; the run-related paths remain unimplemented.

Create workload/version returns 201 with `Location`. Submit/rerun returns 202 with `Location` only after the product transaction durably records the run and pending start command. Cancel returns 202 after its durable command commit; a completed run may win the race. Cancel/rerun accept no body. Repeated cancel has one logical command. Rerun requires confirmed terminal state, copies the original immutable inputs into a new run and links `rerun_of`; it may repeat generation and is distinct from replay.

Responses carry server-generated `X-Request-ID`, `Cache-Control: no-store` and the existing error shape (`error.code`, safe `error.message`, `request_id`). The ID may be empty only if secure ID generation fails. JSON mutation requests use `application/json` with optional UTF-8 charset. Invalid JSON/parameters/header/cursor is 400; valid JSON outside schema/policy is 422; oversized bodies 413; wrong content type 415; conflict 409; dependency unavailable 503. Unknown or inaccessible objects both use 404. Per-operation response codes are defined in OpenAPI. Unknown paths/methods retain F03 behavior.

## Pagination

Collection responses contain `items` and `next_cursor` (null at the end), without a total count. Workloads/runs order by `(created_at DESC, id DESC)`; versions by version number DESC; history by sequence ASC; evidence by ID ASC. Authorization/filtering occurs before pagination. Run listing optionally filters by workload ID.

Cursor encoding is opaque; F06 implements HMAC-signed cursors with 15-minute expiry and a private persistent signing key. They bind caller/collection/filter/order/position, validate integrity and reject mismatched or invalid tokens with 400. The position includes a unique tie-breaker and is applied after owner filtering. Cursors never grant access. Role/authorization checks apply to every subsequent page. There is no cross-page transaction snapshot guarantee; metadata and run status may change during traversal. See [registration conventions](registration-api.md) for the implemented details.

## OIDC and owner checks

The bearer security scheme describes real OIDC API access tokens from the configured Keycloak issuer. Validate signature with trusted issuer keys, exact issuer/API audience, expiry and allowed algorithms; derive identity from `(iss, sub)` and configured role claims. Login uses authorization code with PKCE outside this resource contract. Client owner/role fields are rejected. Missing/invalid token returns 401 with `WWW-Authenticate: Bearer`; a principal without a permitted role returns 403.

Developers read/list/mutate their own records. Operators inspect all owners but cannot mutate another owner's records. Apply policy consistently to nested versions, history, evidence, reports and collection queries. Mutation ownership checks happen before precondition/idempotency lookup. Unauthorized object IDs return 404. A schema-valid permission request is not permission approval; enforce trusted platform source/tool policy when registering a version and again before running activities.

No fake-auth endpoint or static identity header exists. F06 implements the consulted Keycloak client/role mapping and token checks; F07 deploys real login. The original F04 contract alone did not prove security enforcement; see [registration conventions](registration-api.md) for actual validation/revocation behavior and evidence.

## Idempotency and durable execution

Run submission and controlled rerun require a 1–128 character `Idempotency-Key` using ASCII letters/digits/`._:-`. The uniqueness key is validated `(issuer, subject, key)` across both operations. Same key and semantic fingerprint returns the original run and 202; different operation/input returns 409. A matching accepted retry does not re-evaluate dependency availability or terminal status; those checks apply to first acceptance of a new rerun. Preserve keys for the run record's lifetime; do not expire them into accidental resubmission. Unsupported record deletion/retention needs a later decision.

The fingerprint includes the operation and canonical request; rerun includes the parent ID. Whitespace/property order and unordered source selection do not cause a conflict, but question text is preserved. Authenticate/authorize first, then accept the run and pending command in a PostgreSQL transaction with uniqueness enforcement. A hash helper neither creates a durable command nor prevents races; F05 and submission implementation must provide those guarantees. Workload/version registration currently has no idempotency key contract; retrying version POST may create another immutable version. Inspect the result/list after an ambiguous response.

Separate product submission state (`pending`/`dispatched`) and cancellation command state from Temporal execution. Current execution status is either `not_started`, a timestamped Temporal observation, or `unknown` when unavailable with an optional cached last-known observation. The schema rejects unavailable/current-success combinations. PG caches cannot manufacture completion. A persisted report may be available before the workflow's final acknowledgement; artifact availability alone does not claim execution success.

F11 adds approved response consistency rules: delivered cancellation requires dispatched submission; dispatched submission cannot report not_started; and cached state/timestamp are present or absent together. Pending acknowledgement may still have an actual Temporal observation after an ambiguous start. See [the lifecycle contract](workload-lifecycle.md) for transition authority, completion races and the deferred environment boundary.

## Examples and validation

Copy the [workload](../internal/contract/examples/create-workload.json), [version](../internal/contract/examples/create-version.json) and [run](../internal/contract/examples/create-run.json) examples when the runtime endpoints become available. IDs, corpus hash and issuer URL in examples are synthetic, not provisioned resources or chosen Keycloak ports. Set the real returned IDs/approved snapshot/issuer; acquire a real access token; keep it outside Git. Never replay example response identities as authentication.

The contract validator uses pinned [jsonschema v6.0.3](https://github.com/santhosh-tekuri/jsonschema/releases/tag/v6.0.3), asserting JSON Schema 2020-12 formats without network loaders. Tests validate the full document against the vendored [official OpenAPI 3.1 meta-schema](https://spec.openapis.org/oas/3.1/schema/2025-09-15), compile every payload schema, resolve all internal references, validate published examples and exercise rejection/fingerprint/status/report behavior. No framework/router/client generation is introduced; DTO/client generation can be added against this contract when consumers need it.

Run `go test ./internal/contract` and the normal repository checks. Schema validation does not verify stored ownership, source existence, citation hashes, claim support, idempotency transactions or dependency readiness. Those are application/integration tests in subsequent tickets.
