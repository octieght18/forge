# Product and design decision log

Owner: ahmad

Status: D01–D08 accepted on 4 October 2026; F01 review and F02 decisions D09–D14 recorded on 5 October 2026. Baseline thresholds and detailed implementation schemas/configuration require later consultation. Approval is recorded only for explicit responses; delegated selections are identified.

## D01 — First representative workflow

Options presented to the owner:

1. A read-only research agent that uses MCP tools and produces cited evidence.
2. A code-review agent that inspects a repository and proposes findings.
3. An operations agent that investigates an incident through read-only tools.

Impact: determines the developer journey, tool contract, example data, evidence structure, and failure scenarios. Side-effecting workflows, if needed, require a separate decision about permission, approval, and compensation semantics.

Decision: **Read-only research agent using MCP tools and producing cited evidence**, selected by the owner on 4 October 2026. D06 selects a fixed local corpus.

## D02 — Initial customer and isolation boundary

Options presented to the owner:

1. One organization with two isolated teams/projects.
2. One engineering team with multiple workload owners.
3. Multiple organizations with isolated tenants.

Impact: determines who administers access, where quotas apply, the tenant/project model, and which cross-boundary negative tests must pass. Namespace, process, and stronger runtime isolation are architectural choices for F02, not assumptions made here.

Decision: **One engineering team with multiple workload owners**, selected by the owner on 4 October 2026. Multiple organizations are outside the initial customer scope. D05 selects owner-private access and operator visibility.

## D03 — Available weekly capacity

Options presented to the owner:

1. 18 hours total: 8 hours project work and 10 hours study, interviews, applications, and feedback.
2. 10 hours total: 5 hours project work and 5 hours on the parallel activities.

Impact: determines feasible scope and dates. The current backlog's 26-week schedule is a proposal; reduced capacity requires narrower scope or a longer schedule.

Decision: **10 hours total/week: 5 project and 5 study/interviews/job search**, selected by the owner on 4 October 2026. The original backlog estimates 193 project hours: about 39 weeks at this capacity before contingency. D04/D07 select a smaller six-month first release.

## D04 — Scope versus delivery horizon

Options presented to the owner:

1. A smaller first release within six months, with remaining capabilities scheduled later.
2. The full backlog over roughly 40–42 weeks.

Decision: **Deliver a smaller first release within six months and schedule remaining capabilities later**, selected by the owner on 4 October 2026. D07 narrows that release to the API and durable workflow. The old six-month full-scope plan is not a commitment at the accepted capacity.

## D05 — Owner access and collaboration

Options presented to the owner:

1. Developers access their own workloads/runs; the platform operator can inspect all.
2. Workloads/runs are shared within the engineering team; the operator manages policy.

Decision: **Developers access their own workloads and runs; the platform operator can inspect all**, selected by the owner on 4 October 2026. This is an authorization requirement; implementation of identity and isolation remains for F02. Operator visibility does not by itself grant every mutation or tool permission.

## D06 — Research source scope

Options presented to the owner:

1. A fixed local document corpus through a read-only MCP tool, producing a cited report.
2. Public web research through read-only MCP tools, producing a cited report.
3. Both local and public web sources.

Decision: **A fixed local document corpus through a read-only MCP tool, producing a cited report**, selected by the owner on 4 October 2026. Public web retrieval is outside the first demo. Corpus membership, passage identifiers and exact tool schemas are implementation decisions to consult on later.

## D07 — Concrete first-release cut

Proposed for review: workload/version registration; local self-service environments; durable read-only MCP research; authenticated owner-scoped access; basic resource quotas/rate limits; execution history, evidence, usage and audit; safe retry/cancellation; and a documented local quickstart.

Proposed deferrals: cloud deployment, GitOps/progressive rollout, full replay tooling, advanced tenancy and the broader portfolio package. Version registration remains in scope; progressive delivery does not. Controlled retry/rerun and failure inspection remain in scope; a full replay/reset UI does not.

Alternatives offered: add cloud deployment while reducing other scope, or reduce the initial release to API and durable workflow only.

Decision: **Make the first release smaller: API and durable workflow only**, selected by the owner on 4 October 2026. The approved owner-private access, operator visibility and fixed-corpus research behavior remain requirements. Local self-service environment provisioning, quota/rate enforcement, deployment rollout, full replay tooling and broader platform operations move to later releases. A local startup/usage guide is still necessary to demonstrate the API and workflow.

## D08 — Acceptance measures and target-setting

Proposed for review: repeatable failure/authorization tests, valid local-corpus citations, and 10 comparable manual versus self-service provisioning measurements. Set numerical speed/capacity/recovery targets after the F08 baseline rather than guessing them in the brief.

Alternative offered: set numerical performance and recovery targets now.

Decision: **Use the proposed acceptance measures and set performance thresholds after F08**, selected by the owner on 4 October 2026. Apply the failure/authorization and corpus-citation tests to the first release. Because D07 defers environment provisioning, its 10-manual/10-self-service comparison belongs to the later provisioning phase. Do not expand the first release to satisfy that deferred measurement.

## Remaining implementation and baseline decisions

- Quantitative success thresholds and test conditions after the agreed baseline milestone.
- Future side-effecting tools, sharing/ownership transfer and any expansion of operator mutation privileges require a separate decision. F02 accepts the initial owner-only mutation/operator inspection matrix.
- Exact API/tool/evidence schemas, OIDC clients/claims and in-flight revocation semantics, pinned dependencies/images, timeout/retry/request bounds, and model routing eligibility during implementation. F02 establishes the integration boundaries; dynamic policy, Kubernetes provisioning and delivery/operations decisions follow their later phases.
- Retention periods, deployment exposure, cloud budget, and operational recovery objectives.

## Decision process

For each material choice, record the question, options and tradeoffs, the owner's response, and the resulting change to requirements or acceptance criteria. Do not treat silence as approval. Distinguish requirements taken directly from the supplied plan from proposed design choices.

## D09 — First-release foundation

Accepted on 5 October 2026: Go HTTP/JSON with OpenAPI, PostgreSQL product persistence, Temporal durable execution and Docker Compose locally. The owner responded “Accept the recommended foundation”. Public gRPC and both transports were compared; Kubernetes from day one was offered as an alternative.

## D10 — Inference backend

Accepted on 5 October 2026: **OpenRouter**, named by the owner instead of the proposed direct OpenAI backend or local Ollama. F02 selects a narrow HTTP provider adapter, not another agent framework or model-serving system.

## D11 — Deployment target and release timing

Accepted on 5 October 2026: **Local Kubernetes**, explicitly named instead of AWS EKS/GCP GKE. In the follow-up the owner accepted **Docker Compose first; kind later**. The original F02 criterion to choose a cloud target is therefore amended to a local target; managed-cloud selection/provisioning is deferred. Cloud alternatives and dated management-fee costs remain documented for comparison.

## D12 — State, access and MCP boundaries

Accepted on 5 October 2026: PostgreSQL product records/evidence plus a durable start/cancel handoff; Temporal authoritative execution history/status; a worker-owned read-only MCP stdio process and fixed retrieval/generation stages; owner-only mutations and operator inspection. The owner accepted these boundaries while replacing the proposed static identities with **OIDC from the first release**. Architecture records the crash windows, replay/rerun distinction, citation limitations and future failure tests.

## D13 — Local OIDC provider

Accepted on 5 October 2026: **Local Keycloak**, chosen over an existing issuer or local Dex. The consultation included two developer identities and one operator, persistent identity storage, API access-token validation, issuer/subject ownership and authorization-code login with PKCE. This adds identity setup to the first-release quickstart/estimates. Exact client configuration and token/revocation behavior require implementation consultation.

## D14 — Free testing model

Owner instruction on 5 October 2026: **“use a free model from open router for testing”**. The owner delegated selection within this constraint instead of accepting the paid Gemini proposal. Selected `google/gemma-4-26b-a4b-it:free` from the live catalog, with zero prompt/completion token prices and the listed `google-ai-studio` endpoint. This exact slug is an agent selection under delegation, not an explicit owner-named model.

Use public/synthetic fixtures, no paid model/plugin fallback, record actual model/provider identity and validate output locally. Routing/data-policy eligibility is not proven by catalog metadata and must be checked during implementation; failures must not silently broaden routing or spend. Free-tier capacity is not a platform SLO. This documentation task created no credentials, model calls, cloud resources or runtime services.

See [F02 architecture](architecture.md) and [ADRs](adr/README.md) for alternatives, costs, reversibility and verification obligations.

## D15 — Go service foundation and operational conventions

Accepted on 5 October 2026: Go standard library HTTP and structured JSON logs; loopback-only port 8081; separate configuration, HTTP and lifecycle packages; public process-only `/healthz` and `/readyz`; safe JSON errors with stable code/message/request ID. The owner accepted configurable 5s header read, 10s request read, 15s response write, 60s idle and 10s shutdown grace defaults, draining followed by cancellation/connection close after grace, and Windows/Linux CI with Linux race detection.

These operational defaults do not establish performance SLOs. PostgreSQL/Temporal/OIDC integration and product routes follow later tickets. Standard library dependencies avoid a framework/tool dependency at this stage; a future router can replace the transport internals behind the injected handler without changing lifecycle ownership. See [service conventions](service-foundation.md) and [F03 evidence](f03-validation.md).

## D16 — F04 identifiers, concurrency and bounded research profile

Accepted on 5 October 2026: server-issued UUID v4 IDs; revision/ETag checks for metadata updates; 64 KiB mutation bodies; questions up to 4,000 characters; scopes up to 64 documents; immutable versions with ceilings of 16 passages/2,048 output tokens and example settings of 8/1,024. Reports use structured claims with evidence IDs and explicit limitations. These are contract bounds, not F08 performance/quality targets.

## D17 — F04 API shape and release amendment

Accepted on 5 October 2026: `/api/v1/runs` for Forge logical research jobs; contract/Go validation now, persistent endpoints later; only the fixed research workflow in versions, rejecting deferred deployment/resource fields; opaque cursor pages of 20 (maximum 100); run idempotency keys; real OIDC requirements and no fake auth. This explicitly amends the original F04 broad declaration criterion while preserving the deferred capabilities in the backlog. Workloads, immutable versions, run history/evidence/report reads, cancellation and controlled rerun follow the accepted first-release boundaries.

See [API contract](api-contract.md) and [F04 validation](f04-validation.md). Payload validation is not authentication, authorization, durable acceptance or proof of citation support. Cursor signing/lifetime, Keycloak client/role details, database constraints and Temporal/MCP activity parameters remain implementation decisions for consultation.

## D18 — F05 persistence and migration boundary

Accepted on 6 October 2026 in three owner replies: **pgx v5 and explicit SQL**, JSONB for validated immutable specs/inputs with relational ownership and constraints; an explicit `forge-migrate` command with transactional, checksummed, forward-only migrations outside API startup; workloads, versions, runs and durable start/cancel commands now; owner uniqueness, revisions and atomic run/start idempotency; Temporal dispatch, evidence/report storage and HTTP endpoints in later implementation tickets. The owner accepted repository owner checks, separate migration/runtime roles and no RLS initially, plus real PostgreSQL local/CI tests. ORM/other migration tools, immediate artifact storage and first-release RLS were offered alternatives.

F05 implements this boundary against PostgreSQL 18.6 with pinned pgx v5.11.0. Principals remain trusted service inputs until actual OIDC validation exists. The repository cannot authenticate tokens, sign public cursors or deliver commands. See [persistence guide](persistence.md) and [F05 evidence](f05-validation.md).

## D19 — F06 registration, identity and endpoint conventions

Accepted on 6 October 2026 in three owner replies: implement the seven workload/version create/read/list/metadata-update operations, preserving immutable versions and no deletion. Amend original CRUD/idempotency criteria to that F04 scope; registration POSTs have no idempotency guarantee, with run idempotency in F05 and run HTTP endpoints later. The owner selected real Keycloak RS256 access-token validation now, exact issuer/API audience, access-token type checks and developer/operator client roles under `resource_access.forge-api.roles`; five-minute access tokens without per-request introspection, signed-token/JWKS tests now and Keycloak deployment in the local-stack ticket. Immediate revocation/introspection was offered as an alternative.

The owner also accepted HMAC-signed cursors with a private key of at least 32 bytes, 15-minute expiry and caller/collection/filter binding; configurable five-second operation deadlines; PostgreSQL readiness; and operator-managed JSON snapshot/document approvals. F06 retains an explicit health-only default mode and requires complete product configuration to activate registration. See [registration guide](registration-api.md) and [F06 evidence](f06-validation.md).

## D20 — F07 native deployment and retained local identity

On 6 October 2026, the owner instructed **“dont containerize anything”**, superseding both the original F07 Kubernetes card and the earlier Compose-first deployment decision for this stack. Although the same response selected a Docker runtime option, the explicit no-container instruction takes precedence; no Docker runtime was installed. The owner then explicitly accepted **native PostgreSQL, Keycloak and the Go API in the existing Ubuntu WSL environment**, managed by systemd through a PowerShell startup command with verified Windows loopback connectivity. Native Windows services were offered as an alternative.

The owner accepted generated private secrets, separate Forge/Keycloak databases and roles, two developer demo users plus a product operator, a small browser authorization-code/S256 PKCE helper, ordinary teardown that retains data, and a separate explicit deletion operation. Temporal follows its workflow ticket. The local stack retains exact issuer/subject ownership, API loopback binding and same-origin JWKS validation; no container networking exceptions were added. Resource caps and startup/readiness deadlines are local operational defaults, not F08 performance thresholds.

See [native stack guide](local-stack.md) and [F07 evidence](f07-validation.md). Real Keycloak deployment replaces F06's fixture-only login limitation for this local setup; production TLS/identity operations, backup recovery, Temporal and Kubernetes remain later work.

## D21 — F08 measured baseline scope and protocol

Accepted on 7 October 2026 in two owner replies: measure the current native startup and authenticated workload/version registration journey, document the later manual-versus-self-service provisioning procedure, and keep human onboarding/typing in a separate unmeasured worksheet until a person performs it. This amends the original F08 manual-environment criterion to match the accepted API/workflow release.

The owner accepted five retained-data stop/start trials with cached prerequisites, including build, migrations and readiness; ten sequential authenticated registration/read journeys; and one occupied-port failure/recovery case. Preserve raw timings, failures, commands and PC/WSL conditions; summarize median and range without declaring SLOs. Retain existing data and the ten synthetic workload/version fixtures. The original ten-manual/ten-self-service provisioning comparison stays deferred. Numerical speed, capacity and recovery targets remain pending owner review after the observations. See [procedure](baseline-procedure.md), [human worksheet](baseline-human-worksheet.json) and [results](f08-baseline.md).
