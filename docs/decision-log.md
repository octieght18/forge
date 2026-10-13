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

## D31 — F17 in-process OpenTelemetry views

On 11 October 2026 the owner asked to implement the next backlog ticket, F17. The ticket asks for OpenTelemetry measurements of provisioning duration, success and failure counts, retry reasons, a minimal metrics view, and request and operation identity on traces. The architecture already places a collector and dashboard later. This change does not add either one, and it does not enable the controller-runtime metrics server.

The API and the environment controller each keep a process-local view: outcome counts, durations, and a bounded reason vocabulary. `GET /api/v1/telemetry` returns the API process view to an operator. Reconciliation reasons stay in the controller process because that process has no scrape Service. API spans continue an incoming W3C trace and attach the server request ID and the operation ID. Controller passes do not inherit the caller's request context. Tokens and corpus text are not attributes.

See the [telemetry guide](telemetry.md) and [verification](f17-validation.md).

## D32 — F18 controller failure recovery

On 11 October 2026 the owner asked to implement the next backlog ticket, F18. The ticket asks for convergence after an API, controller, or Kubernetes failure: one owned boundary, a stable final state, a measured recovery time, and reconciliation failure and success counts.

The controller already repairs missing children and an identity-lookup outage. F18 adds one sequence that uses those paths together. The API failure is the controller's identity lookup, which is how this controller reaches the product records. The registration HTTP server is not stopped. The controller failure is a new reconciler using the same client after the first one stops. The Kubernetes failure deletes the quota and limit range after the namespace exists. The test then requires one namespace, one quota, one limit range, and one worker account, each owned by the environment, and an unchanged environment resource version and namespace UID on the next pass.

Recovery in that process was 7.1103ms, with one reconciliation failure (`IdentityUnavailable`) and two successes. The test identity lookup waits one millisecond so a Ready pass that this machine's clock would report as zero still produces a duration sample. Production lookup does not wait. A zero reading stays omitted from the histogram.

An operation that already observed `Failed` stays `failed` after the boundary becomes ready. A later operation can record `ready`. The installed controller still does not write operation rows. Whole-node restart remains the F13 kind smoke. No run route, collector, or second orchestrator is added.

See the [recovery verification](f18-validation.md).

## D33 — F19 three-template onboarding measurement

On 11 October 2026 the owner asked to implement the next backlog ticket, F19. The ticket asks to onboard a service, a tool-calling agent, and a resource-constrained job, measure provisioning time and manual steps against the F08 baseline, and record the supported workload count and failure recovery.

The versioned API still accepts only `research.v1` with `corpus.search` and `corpus.read`. The job template uses that contract and records the controller profile `small-v1`. Rendering does not apply a quota or start a Kubernetes Job. A developer can register, deploy, and request provisioning for each kind. The request returns `202` before a namespace exists. Applying the environment remains the operator command from [D27](#d27--restored-environment-controller-and-workload-boundaries).

Three requests on a disposable PostgreSQL measured provision medians below. They are a different path from the F08 native loopback samples, so they are not a speedup claim. The ten paired manual trials in [D08](#d08--acceptance-measures-and-target-setting) stay unclaimed. The human worksheet stays unmeasured. The controller recovery check from [D32](#d32--f18-controller-failure-recovery) was run again with this ticket.

See the [onboarding measurement](f19-validation.md).

## D34 — F20 self-service contract

On 11 October 2026 the owner asked to implement the next backlog ticket, F20. The ticket asks for the API-to-controller flow, lifecycle transitions, the tradeoff between namespace isolation and stronger isolation, and the known cleanup and recovery limits.

The contract documents the path that already exists. A developer accepts a provisioning operation. An operator applies `ForgeEnvironment`. The controller converges one namespace per workload and repairs that boundary idempotently when it owns the objects. The installed controller still cannot write the operation row, so a live `Ready` namespace and an `accepted` operation can both be true until an observation is recorded. Namespace isolation with `small-v1` is the current boundary. A separate cluster, a sandbox runtime, and NetworkPolicy enforcement remain later decisions. Cleanup holds the environment finalizer until the namespace is gone, and it refuses persistent storage and foreign finalizers.

No new API route, controller permission, or isolation mechanism is added. See the [self-service contract](self-service-contract.md) and [verification](f20-validation.md).

## D35 — F21 execution, tool, and evidence contracts

On 11 October 2026 the owner asked to implement the next backlog ticket, F21. The ticket asks for tool permissions, an MCP invocation contract, typed failure classes, a split between deterministic workflow decisions and nondeterministic activities, and a model of request, execution, steps, evidence provenance, and uncertain results.

The card lists G2 as a prerequisite. G2, the measured self-service provisioning gate, is still on the Backlog. This design does not mark that gate complete. It uses the research contract already accepted for the API: `research.v1`, the two read-only corpus tools, and the run request and report outcomes from F04 and F11.

The new payloads are `MCPInvocation`, `MCPResult`, `ActivityFailure`, `WorkflowDecision`, and `ActivityResult`. An empty tool result and an `insufficient_evidence` report are successful uncertain outcomes. A `tool_denied` result is an activity failure. Workflow decisions carry no corpus text. Public run routes, Temporal workers, and the MCP process remain later tickets.

See the [execution contract](execution-contract.md) and [verification](f21-validation.md).

## D36 — G2 measured self-service provisioning

On 11 October 2026 the owner asked to implement G2. The gate requires idempotent reconciliation with a measured recovery, a published workload count and setup reduction, and a developer environment request that does not add a manual infrastructure step beyond the documented prerequisites.

F19 and F20, the cards blocking the gate, are already done. F18 measured recovery at 7.1103 ms and a later run measured 9.4041 ms, each with 1 reconciliation failure and 2 successes. A repeated ready pass keeps the environment resource version. F19 onboarded 3 workloads. The provision median on that disposable database was 4.041 ms. Login and the token file remain prerequisites. `environment.ps1 apply` remains available to operators and is no longer required for the developer request.

The controller leader reads open operations and submits one `ForgeEnvironment` for `provision`, or removes it for `delete`. A second submit with the same owner leaves the object in place. A different owner is refused. The environment role gains SELECT on the operation columns it needs and stays read-only, so it still cannot mark the operation `ready`. The API process still has no Kubernetes credential. The developer receives no kubeconfig. The running cluster uses this behavior after its controller image, grant, and role are updated to this commit.

See the [gate record](g2-validation.md).

## D37 — F22 Temporal execution workers

On 12 October 2026 the owner asked to implement the next backlog ticket, F22. The ticket asks for durable workflow IDs, correlation identity, control-plane records that do not compete with Temporal, a local Temporal deployment, and a Go workflow and activity worker.

The run already stores `forge-run/<run UUID>`. The dispatcher starts that ID with duplicate reuse rejected, and a later start describes the same execution. The workflow copies the run, workflow, and command IDs into each activity. PostgreSQL still stores the run and the command delivery. Temporal history is the execution record. No execution-status column was added.

The worker's retrieval activity does not call a corpus tool. Generation returns `model_unavailable`, and the workflow completes with `fail_closed`. MCP adapters and model calls remain later tickets. The local dev server is `scripts/temporal-dev.ps1`. The native stack does not launch it. Public run routes stay absent.

See the [worker guide](temporal-worker.md) and [verification](f22-validation.md).

## D38 — F23 context resolution and MCP tool adapter

On 12 October 2026 the owner asked to implement the next backlog ticket, F23. The ticket asks for validated tool inputs, authorization before a side effect, redacted credentials, stored context and evidence with provenance and retention intent, one MCP adapter, and one deterministic test tool.

The dispatcher copies the frozen question, snapshot, and document allow list into the workflow. A document outside that list fails before another execution starts. The retrieval activity checks the invocation schema again and only then starts `FORGE_MCP_COMMAND`. The child process keeps `PATH` and does not inherit the worker's other environment. Bearer tokens, database URLs, and `password=` text are removed from tool errors.

The fixture answers `corpus.search` and `corpus.read` with one synthetic platform-brief passage. Accepted passages are stored with the run, workflow, snapshot, passage, excerpt hash, and retention `workflow_history`. That retention is the Temporal history period. No product evidence table or expiry clock is added. Generation still returns `model_unavailable`.

See the [adapter guide](mcp-adapter.md) and [verification](f23-validation.md).

## D39 — F24 model execution and evidence-based results

On 12 October 2026 the owner asked to implement the next backlog ticket, F24. The ticket asks for a completed workflow that resolves context, calls tools, and persists a result, with evidence links, model identity, usage, and uncertainty status. Tests must use fixtures, and the backend choice and secret handling must be documented.

The hosted backend remains the OpenRouter route selected in F02: `google/gemma-4-26b-a4b-it:free`, with provider data collection denied and no paid fallback. `FORGE_MODEL_API_KEY` is read from the worker environment and sent only as an Authorization header. The workflow tests use a deterministic fixture instead of that network call. `FORGE_MODEL_FIXTURE=true` selects the same fixture when no key is present.

An answered report cites only evidence IDs returned by the authorized search. No passages produce `insufficient_evidence`. The workflow result in Temporal history is the durable copy. The worker process also keeps the report in memory. No product report table or public run route is added.

See the [model guide](model-execution.md) and [verification](f24-validation.md).

## D40 — F25 retry classification and safe compensation

On 12 October 2026 the owner asked to implement the next backlog ticket, F25. The ticket asks for retryable and nonretryable classes, backoff limits, idempotent activity keys, an injected duplicate delivery, a dependency failure halfway through a run, and compensation for one partial side effect.

`tool_unavailable` and a failed call to a configured model may be attempted again. `tool_denied`, `model_invalid`, and identity mismatches may not. A missing model configuration finishes the activity and the workflow records `fail_closed` without spending the retry budget. Retryable activities have three attempts, starting at one second and doubling to a four-second ceiling. That budget is separate from the command reconciler's eight delivery attempts.

A successful retrieval or generation is stored under a key from the run, the step, and the frozen context. A duplicate delivery returns that observation and does not call the tool or the model again. Evidence written by a search that never becomes a report is the compensated side effect: its retention becomes `compensated`, and a report for that run is dropped. Compensating the same run again does not change the record. This does not reverse a hosted model charge.

See the [retry guide](retry-compensation.md) and [verification](f25-validation.md).

## D41 — F26 timeout, cancellation, and long-running status

On 12 October 2026 the owner asked to implement the next backlog ticket, F26. The ticket asks for cancellation propagated to tools and context deadlines, persisted safe terminal states, an explanation when compensation does not finish, and activity or workflow timeouts with a heartbeat where a call can wait.

The workflow execution deadline is 2 minutes. Each activity must finish within 30 seconds and heartbeat within 10 seconds. Retrieval and generation record a heartbeat before the tool or model call. The tool process uses that activity context, so cancellation or a deadline stops it. A canceled context is not retried as `tool_unavailable`.

After cancellation, a disconnected context compensates leftover evidence and records `canceled`. The compensation note is `compensation_completed` or `compensation_incomplete`. The run stays canceled in either case. Incomplete compensation means the evidence may still say `workflow_history`. Temporal history is the record a restarted worker can see. The in-process list is not. No public status route or product execution-status column is added.

See the [timeout guide](execution-timeouts.md) and [verification](f26-validation.md).

## D42 — F27 execution history API

On 12 October 2026 the owner asked to implement the next backlog ticket, F27. The ticket asks for steps, attempts, timestamps, and safe errors, and for start, cancel, history, and result endpoints with an auth abstraction. It also asks to record that full OIDC and RBAC validation is the M4 security gate.

Registration already verifies Keycloak access tokens: issuer, audience, signature, expiry, and the developer and operator roles under `resource_access.forge-api.roles`. F27 uses that verifier. It does not add a second authenticator. Developers read and cancel their own runs. Operators can read and cannot mutate another owner's run. Tenant RBAC remains the M4 gate.

Start and cancel commit the existing PostgreSQL run and command records. History is sanitized Temporal events. The result is the terminal workflow value: decision, a known reason, outcome, and evidence identifiers. Prompts, corpus excerpts, and bearer tokens are not copied from activity payloads or failure messages. `GET /report` and `GET /evidence` stay closed because those schemas require claim text and excerpts, and no product table stores them. Run list and rerun stay closed.

The API dials Temporal only when `TEMPORAL_ADDRESS` is set. The native stack does not set it. Without a client, history and result return 503 and the run read reports an unavailable execution. No execution-status column was added.

See the [history guide](execution-history.md) and [verification](f27-validation.md).

## D43 — F28 replay and evaluation fixtures

On 12 October 2026 the owner asked to implement the next backlog ticket, F28. The ticket asks to distinguish replay, retry, and re-execution, to default debugging to fixtures, and to cover a success, an uncertain result, and a failed step.

Replay uses Temporal's workflow replayer. Activity functions are not entered, so the model and MCP are not called. Retry remains the F25 activity policy. Re-execution is `POST /api/v1/runs/{run_id}/rerun`: a new run and workflow ID linked by `rerun_of`. It is not a replay of the original history.

`FORGE_DEBUG=true` selects the in-process corpus fixture and the fixture model and ignores a configured MCP command or model key. The SDK test environment does not export history, so the replay test builds a history and does not claim a download from a live server. No Temporal dev server was started.

See the [mode guide](replay-modes.md) and [verification](f28-validation.md).

## D44 — F29 durable recovery measurements

On 12 October 2026 the owner asked to implement the next backlog ticket, F29. The ticket asks for attempt counts, no duplicated side effects, cancellation state, measured recovery time, and a worker killed mid-execution and resumed.

The Temporal CLI is not installed here and the dev server script was not started. A live process kill was not run. The SDK test suite is the measurement. A flaky tool fails once and then succeeds: retrieval has two attempts, one evidence row is stored, and compensation marks that row rather than adding another. An uncertain fixture result stores no evidence. Cancellation records `canceled`. A replayer started afterward does not enter activities. The one-second retry interval was skipped by the test clock; the recorded wall times are one sample, not an SLO.

See the [verification](f29-validation.md).

## D45 — F30 workflow and replay guide

On 12 October 2026 the owner asked to implement the next backlog ticket, F30. The ticket asks for replay safety, operator procedures, the recorded failure timeline, current limitations, and an explanation of retries, compensation, uncertainty, and evidence storage.

The guide repeats the F29 sample rather than a new measurement: 43361 µs and two retrieval attempts for the dependency failure, one compensated evidence row, 12993 µs for `insufficient_evidence`, 1816 µs for `canceled`, and 535 µs for a replayer that did not enter activities. The one-second retry was skipped by the test clock. No production SLO is claimed. The dev server was not started and a live worker was not killed.

See the [guide](workflow-replay-guide.md) and [verification](f30-validation.md).

## D46 — F31 tenant threat model

On 12 October 2026 the owner asked to implement the next backlog ticket, F31. The ticket lists G3 as a prerequisite. G3 is still an open gate. This decision does not mark G3 done.

The design keeps the accepted customer: one engineering team, developers on their own records, operators reading all records and not mutating another owner's. Keycloak already checks issuer, audience, signature, expiry, and the developer and operator roles. The MCP command is one operator-configured executable, checked against the frozen document list before the process starts. The environment controller remains privileged infrastructure and does not hand out a kubeconfig.

The model lists spoofing, confused deputy, cross-tenant data access, SSRF, and tool abuse, and it separates tested demo controls from the missing tenant boundary and from production requirements. A tenant and a project are defined here. They are not columns yet. No production isolation is claimed.

See the [threat model](threat-model.md) and [verification](f31-validation.md).

## D47 — F32 tenant and project boundaries

On 12 October 2026 the owner asked to implement the next backlog ticket, F32. Existing records are owned by issuer and subject. The forward migration assigns them the demo tenant and demo project and does not delete them. Workloads, versions, and runs store those identifiers, and repository reads and mutations require the caller's tenant and project. Operators still read other owners inside that scope and cannot mutate them. A developer or operator read of another tenant's id is 404.

There is no product evidence table. Evidence stays reachable only through a scoped run. This is not production tenant isolation, and G3 stays open.

See the [verification](f32-validation.md).

## D48 — F33 OIDC and workload identity

On 12 October 2026 the owner asked to implement the next backlog ticket, F33. User-token verification already exists. It was not replaced. The same Keycloak checks still require issuer, audience, RS256 signature, and expiry. Forged, wrong-audience, and expired tokens stay rejected. The existing rotation test still accepts a new signing key and rejects an unknown signer.

A service token is `identity_kind=service` with role `service` and no developer or operator role. Product routes return 403. A workload identity is built from the stored run and workflow id. A bearer token that claims to be a workload is rejected. This does not add production revocation or close G3.

See the [identity guide](identity.md) and [verification](f33-validation.md).

## D49 — F34 RBAC and tool authorization

On 12 October 2026 the owner asked to implement the next backlog ticket, F34. The tools stay `corpus.search` and `corpus.read`. Operators still cannot mutate another owner's workload. A denied tool does not start a process.

The matrix is developer, operator, and service. Reads and mutations already require the caller's tenant and project. Provision stays a developer action. Telemetry stays an operator read. A service token and a mixed-role token are rejected. `corpus.write` is rejected before `exec`. Cross-tenant run ids stay 404. The activity does not carry a second tenant check; the API is the admission point. G3 stays open.

See the [matrix](authorization.md) and [verification](f34-validation.md).

## D50 — F35 quotas and rate limits

On 12 October 2026 the owner asked to implement the next backlog ticket, F35. `small-v1` already caps one namespace. This change adds API admission limits and does not replace that profile.

A tenant gets 200 requests per second in one API process, 100 workloads, and 100 pending start commands. The workload and pending-run counts take a transaction lock in PostgreSQL, so callers that share that database observe one cap. The request window does not leave the process. A limited request is 429 with `Retry-After: 1` and the existing error object. No distributed quota consensus is claimed.

See the [verification](f35-validation.md).

## D51 — F36 audit and usage metering

On 12 October 2026 the owner asked to implement the next backlog ticket, F36. Audit events record the actor subject, tenant, operation, time, target path, and outcome. They do not record tokens, prompts, or corpus text. Model units are the prompt and completion token counts already on the fixture report. Tool calls and those token counts are added once per activity idempotency key. The fixture execution is the reconciliation source. The counters live in the process, not in a second database.

See the [verification](f36-validation.md).

## D52 — F37 secrets and data retention

On 13 October 2026 ahmad asked to implement the next backlog ticket, F37. The ticket asks for a retention and deletion policy for executions, evidence, audit, and attachments, a safe demo cleanup, and a statement of where secrets live.

Executions stay in PostgreSQL and in Temporal history. Principals, versions, and runs already reject DELETE. Evidence and audit stay in process memory and are not product tables. The API does not accept attachments. Secret values stay in environment variables, Kubernetes Secret objects, or local files under the retained state directory. They are not written into Git, specs, or logs.

The demo cleanup is `forge-demo-cleanup`. It deletes only regular files named `demo-synthetic-*` in an explicit directory after `--confirm delete-synthetic-demo-records`. It refuses a `forge-native` or `forge-kubernetes` path, a directory that contains `PG_VERSION` or a secret file, a secret-shaped name, and a symlink. It does not take a database URL. Native purge remains a separate operator action and is not this job. A backup stored with the retained state is outside the cleanup.

See the [retention guide](retention.md) and [verification](f37-validation.md).

## D53 — F38 network and policy-as-code

On 13 October 2026 ahmad asked to implement the next backlog ticket, F38. The architecture already preferred Kyverno for Kubernetes resource policy, and OPA for later dynamic API policy. This ticket uses Kyverno. It does not add an OPA service or a second authorizer.

The ClusterPolicy rejects privileged containers, privilege escalation, host namespaces, and host path volumes. Kyverno is not installed in this session. Docker, kind, and kubectl were not available. The rejection that ran is the CI checker over the policy documents and the kind resource generator.

The network documents are default-deny for `forge-local`, same-namespace ingress and egress, DNS to `kube-dns`, and TCP 443 for the model route. Corpus tools stay stdio and have no network rule. Kind's current network plugin does not enforce NetworkPolicy. The manifests were not applied. No denied network path was observed.

See the [policy guide](policy.md) and [verification](f38-validation.md).
