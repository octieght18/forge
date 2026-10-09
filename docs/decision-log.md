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

## D22 — F10 cancellation and reduced registration profiling

Accepted on 7 October 2026: real HTTP tests for client cancellation, PostgreSQL cancellation/rollback, OIDC/JWKS waits and graceful draining; an isolated disposable PostgreSQL database and signed OIDC fixtures for profiling; real Keycloak login stays covered by native CI. Profiles are files, with no public profiling endpoint. After asking for a smaller load, the owner accepted one and two concurrent clients, five seconds of warmup and fifteen seconds of measurement per level, preserving the four-request registration/read journey, raw latencies/statuses, CPU/heap profiles and conditions. Results remain descriptive without SLO or capacity targets.

The API-level JWKS disconnect regression exposed unread HTTP/1 bodies delaying cancellation until the two-second shared key-fetch timeout. The owner explicitly accepted a bounded pre-authentication body read: at most 64 KiB under the existing five-second operation deadline, with media-type, JSON/schema and product validation still after authentication. Ordinary unauthenticated requests retain 401; oversized or failed reads can return 413/400/503 before authentication. This changes input-error precedence deliberately. Shared JWKS requests remain independently bounded so one caller cannot cancel another caller's key fetch. See [profiling procedure](profile-procedure.md).

For publication, the owner explicitly chose **code/report only; keep raw profiles local**. Raw records, profiles, detailed machine files and raw test outputs are retained locally under ignored docs/profiles/ and an unpushed local evidence branch; the public repository receives the code, procedure and summarized report/results. No raw evidence blobs are included in the pushed main history.

## D23 — Balanced local performance acceptance limits

On 7 October 2026 the owner selected **Balanced** after reviewing measured F08/F10 results. Accept each retained-data native startup within 90 seconds; each native four-request registration/read journey within 150 ms; request p95 within 5 ms separately at one/two clients in the full F10 profiling harness; occupied-port detection within one second; and readiness within 90 seconds after release of the occupied-port fixture. Require zero unexpected errors in healthy scenarios and passing persistence/cancellation checks. Relaxed and Stretch were offered alternatives; the Balanced limits are explicitly owner-approved.

Apply these as inclusive local repeat-test limits under the accepted F08/F10 protocols and comparable conditions on this PC. Keep the native resource-capped real-OIDC journey separate from the uncapped signed-fixture profiling process. No production availability SLO, capacity ceiling, fresh-installation or crash/backup recovery target is selected. The prior numerical-target deferral in D08/D21/D22 is resolved for these five local timings; broader capacity, workflow and production targets remain future decisions. Original observations and failed attempts stay intact, and F10 raw profiles remain local under D22.

See [accepted limits/procedure](performance-targets.md), [machine-readable targets](performance-targets.json) and [retrospective recorded-observation assessment](validation/balanced-target-assessment.json). All accepted local timing and healthy-operation checks pass the recorded evidence. This is assessment of existing observations after approval, not a new load/startup measurement.

## D24 — F11 research specification and lifecycle

Accepted on 8 October 2026 in two owner replies: adapt the original F11 environment specification/lifecycle ticket to the fixed research specification and run/command lifecycle, document the deferred environment boundary, and amend its checklist. Keeping F11 as a future environment-only design was offered as an alternative. Resource/image/deployment policy, Kubernetes generation/conditions and Pending/Provisioning/Ready/Failed/Deleting remain deferred under the smaller API/workflow release and native no-container deployment.

The owner accepted three response safeguards: delivered cancellation requires acknowledged start; acknowledged start cannot report not_started; and a cached Temporal state must include its observation timestamp, with both absent if there is no cache. Pending acknowledgement may coexist with a Temporal observation after an ambiguous start. Retries/cancellation remain intents; run endpoints and dispatch follow later tickets. This preserves F04's separate submission, cancellation and authoritative Temporal execution axes, immutable versions, ownership and research ceilings rather than adding another execution state machine.

See [specification and lifecycle](workload-lifecycle.md), [OpenAPI](../internal/contract/openapi.json) and [F11 evidence](f11-validation.md). The stricter planned response schema affects future run consumers; existing registration payloads and database migrations are unchanged. No additional dependency, container or model call is introduced. Observation truth/freshness, command acknowledgements and safe dispatch after history retention need implementation verification later.

## D25 — F12 durable command reconciliation

Accepted on 8 October 2026 in two owner replies: adapt F12 from the deferred Kubernetes CRD/controller to durable start/cancel command reconciliation. Leaving F12 deferred and moving to F21 contracts was offered as an alternative. Preserve the original CRD/schema/watches/conditions/ownership-reference requirements as deferred work and amend F12's title/checklist; the real Temporal adapter and dispatcher deployment remain F22 work.

The owner accepted a separate privileged dispatcher database role, short PostgreSQL claims with expiring fenced tokens, one command at a time per worker, 30-second leases and five-second operation deadlines, eight durable attempts with 1–60-second backoff then parking for operator investigation, start acknowledgement before cancel delivery, a forward migration, and real PostgreSQL concurrency/crash/fencing tests. No dispatcher service starts until F22. A one-second polling interval is the local loop default, not a throughput target.

F12 supplies the store/reconciler libraries and narrow verified backend boundary; schema/identity/scope checks precede delivery, and receipt identity must match before acknowledgement. Runtime producer grants cannot set delivery metadata; native setup upgrades credentials without rotating existing values and prepares the separate dispatcher role without exposing it to the API. Retries persist safe failure codes, never raw backend details. Parking leaves durable pending intent and cannot fabricate Temporal failure. Operator reset tooling remains future work.

See [implementation/upgrade guide](command-reconciliation.md) and [verification](f12-validation.md). Lease fencing protects database writes, not exactly-once external effects; backend idempotency, context compliance, ambiguous-start verification and history-retention behavior require real Temporal tests in F22. Public run endpoints, status cache, detailed delivery observability and Kubernetes provisioning are not added.

## D26 — Containerized local Kubernetes deployment

On 8 October 2026 the owner requested containerization for Kubernetes inside WSL, explicitly superseding D20's no-container direction. In three subsequent replies the owner accepted a single-node kind deployment of the current API, PostgreSQL and Keycloak; copying existing product and identity databases while retaining native originals; and preserving the loopback API/login URLs and exact OIDC issuer with fixed proxy sidecars/port forwarding. API-only packaging with native databases, fresh demo data and a new HTTPS issuer were offered alternatives.

Use a non-root static API/migration image, upstream digest-locked PostgreSQL/Keycloak/NGINX images and kind node, explicit migration Job, separate runtime/identity/migration secrets and retained WSL-backed PostgreSQL storage. Stop native writes, make private logical backups and verify all restored table data before starting identity/API services. Preserve signing keys, owner subjects, cursor key and corpus policy. Keep the API's loopback bind guard; a fixed NGINX sidecar connects Pod networking and the original issuer to Services. No external ingress or registry publication is added.

This changes deployment packaging now; it does not revive the unimplemented CRD/controller or widen the fixed research API. Workload records are declarations, with execution worker containers following their implementation tickets. Local Keycloak dev mode/HTTP, single-node storage, administrator-readable Secrets and default kind networking remain explicit limitations. Native Balanced timing thresholds need a separate Kubernetes baseline before reuse. See [local Kubernetes guide](kubernetes-local.md) and [verification](kubernetes-validation.md).

## D27 — Restored environment controller and workload boundaries

Accepted on 8 October 2026: restore the deferred CRD/controller prerequisite, tracked separately as F12a, then implement F13 with one namespace per workload. Keep completed F12 command reconciliation intact. The owner accepted resource/runtime boundaries first and deferred enforced network isolation; no kind networking rebuild is authorized by this decision.

In three subsequent replies the owner accepted pinned Kubernetes-1.37-compatible Go controller-runtime, a separate restricted controller Pod with watches/conditions/bounded requeues; an operator-only cluster-scoped ForgeEnvironment with immutable registered workload/owner and fixed small profile, verified using a new read-only PostgreSQL role; and two-Pod quotas of 1 CPU/1 GiB requests and 2 CPUs/2 GiB limits, defaults of 100m/64 MiB requests and 500m/512 MiB limits, Restricted admission and token-free worker accounts without RBAC grants. Cleanup waits, rejects conflicting ownership or persistent storage, and preserves foreign finalizers. Ready describes the boundary, not worker execution.

This revives the distinct environment design deferred in D24/D25 beyond D26's packaging scope, without widening research versions or adding a public environment API. Operator provisioning, direct client-go and immediate API integration were offered alternatives. No agent image or Temporal adapter is implemented here. See [environment guide](execution-environments.md) and [validation](f13-validation.md).

## D28 — F14 developer CLI over the registration API

On 9 October 2026 the owner asked to implement the next backlog ticket, F14. The versioned API still has no delete, rollout, run, or public environment operation. The CLI therefore maps `register` to workload creation, `deploy` to immutable version registration, `status` to workload and version reads, and `delete` to the API's existing rejection. No product deletion, Kubernetes credential, or execution endpoint is added.

`deploy` reports `execution: not_started` and `environment: not_requested`. Operator boundary apply/delete stays the separate environment command. See the [developer CLI guide](developer-cli.md) and [verification](f14-validation.md).

## D29 — F15 research service and agent templates

On 9 October 2026 the owner asked to implement the next backlog ticket, F15. The registration API still accepts only the fixed research workflow, so both templates use that contract. `service` is the registrable workload and version. `agent` adds the MCP tool names in its description and a sample run body. Health, telemetry, and the existing environment manifest shape are included as local files. No credential, collector, public environment endpoint, run route, or second orchestrator is added.

See the [template guide](workload-templates.md) and [verification](f15-validation.md).

## D30 — F16 asynchronous provisioning operations

On 9 October 2026 the owner asked to implement the next backlog ticket, F16. The ticket asks for a prompt accepted operation, a status URL, terminal errors, conflict and timeout handling, and desired state that stays consistent with controller observation. The current product still has no public namespace mutation API and does not give developers a kubeconfig. [D27](#d27--restored-environment-controller-and-workload-boundaries) keeps cluster apply on the operator command and the controller's database role read-only.

F16 therefore adds a durable operation record on the authenticated API. `provision` and `operation delete` return `202` with a status path. A second open operation, or a version that is not the workload's current version, conflicts and changes nothing. Cancel requests deletion and is rejected once the operation is finished. A deadline moves an unfinished operation to `timed_out` with a terminal error. Controller phases `Ready`, `Failed`, `Deleting`, and `Absent` update the same record through one comparison function. The installed controller reports those phases only when a reporter is configured; the read-only database role does not gain a write grant, so a running kind controller still publishes status on the environment object rather than in this table.

No run route, worker image, or second orchestrator is added. See the [operation guide](provisioning-operations.md) and [verification](f16-validation.md).
