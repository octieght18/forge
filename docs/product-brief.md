# F01 — Internal platform product brief

**Status:** Accepted by the owner on 5 October 2026; product decisions D01–D08 accepted  
**Product:** Forge (working name inherited from the Wekan project)  
**Owner:** ahmad  
**Prepared:** 4 October 2026  
**Intended audience:** Internal engineering stakeholders, platform operators, and application developers

## 1. Internal request

Our engineering teams need a consistent way to register, deploy, and operate AI workloads and agents. Today, each workload risks becoming a separate integration of execution infrastructure, identity, tool access, resource controls, and debugging conventions.

We request a small, reusable control plane that lets developers describe a workload and consume a documented self-service path while operators enforce policy and investigate failures. The platform must make asynchronous execution and failure recovery understandable, and must produce evidence of developer impact and operational behavior.

This problem statement is a requirement derived from the supplied project plan. Actual organizational pain, current manual steps, users, and baseline measurements must be validated; this brief does not claim they have already been observed.

## 2. Product objective

An application developer can register an AI workload, declare its tools, permissions and resource requirements, request an isolated environment, start an asynchronous workflow, and retrieve the result with execution history and evidence. A platform operator can govern access and capacity, inspect an audit trail, roll out versions, and recover from representative failures.

Success means demonstrable self-service behavior, bounded security claims, durable execution, and measured tradeoffs. Go service engineering and Kubernetes-native reconciliation are explicit learning and evidence goals from the supplied plan.

## 3. Users and jobs to be done

| User | Job to be done | Desired outcome |
|---|---|---|
| Platform operator | Configure platform policy, boundaries, resource budgets, and supported workload contracts | Developers consume a consistent platform while the operator retains visibility and control |
| Platform operator | Investigate a failed provisioning operation, execution, or release | Identify the failing component and recover through documented, tested procedures |
| Application developer | Register and version a service or agent without handcrafting infrastructure for every workload | A clear API/CLI path creates and manages the requested environment |
| Application developer | Run a workflow that calls permitted tools and produces evidence | Observe progress, handle uncertainty, and receive a reproducible result |
| Application developer | Debug a partially completed execution | Inspect steps and attempts, distinguish replay from rerun, and avoid duplicate side effects |

The initial customer is **one engineering team with multiple workload owners**, approved in D02. Multiple customer organizations are outside the initial customer scope. D05 approves developer access to their own workloads/runs and operator visibility across all owners. Unauthorized reads and writes must be denied consistently, including history, evidence, usage and cancellation endpoints. The exact action matrix will be consulted on later; visibility does not imply unrestricted operator mutation or tool permissions.

## 4. Scope and release boundaries

The following capability set is the longer-term product outcome supplied by the owner. D04 approves a smaller six-month first release, and D07 narrows it to the API and durable research workflow. The full table is not a promise that every capability ships within six months.

| ID | Required capability | Observable acceptance evidence |
|---|---|---|
| FR01 | Register and version an AI workload or agent | A valid request creates a stable workload identity and version; invalid requests return actionable errors; the record survives a service restart |
| FR02 | Declare tools, permissions, resource limits, and deployment policy | A stored version describes these controls; disallowed requests are rejected at documented enforcement boundaries |
| FR03 | Provision and manage an isolated execution environment | A request reports observed lifecycle state; repeated reconciliation converges without duplicate resources; cleanup follows ownership rules |
| FR04 | Run asynchronous and long-running workflows | The request returns an execution identity before completion; progress and terminal state remain inspectable across worker interruption |
| FR05 | Expose a versioned, authenticated API | Published API examples succeed for authorized identities; invalid identities and forbidden actions are rejected; any initial auth placeholder is explicitly marked |
| FR06 | Enforce quotas and rate limits | Excess requests or resource allocations produce clear outcomes; concurrent requests cannot silently exceed the declared enforcement policy |
| FR07 | Record usage and execution history | Execution steps, attempts, timestamps, outcomes and attributable usage are retained with documented units and deduplication behavior |
| FR08 | Retry safely after failures | A transient dependency failure recovers according to bounded retry rules; duplicate delivery does not repeat a tested non-idempotent side effect |
| FR09 | Roll out workload and platform versions | An explicit version is deployed and its history recorded; a representative bad rollout can be restored through a tested rollback path |
| FR10 | Debug a failed execution through replay or controlled rerun | A failed-step example can be reproduced; replay, retry and re-execution semantics and their side-effect limitations are documented |
| FR11 | Produce an audit trail | Representative permission-sensitive actions record actor, boundary, target, timestamp and outcome without disclosing credentials |
| FR12 | Provide documented self-service deployment and onboarding | A clean-environment walkthrough provisions the platform, registers a workload, runs it, inspects a result, and documents remaining manual steps |

### Accepted first release — API and durable workflow only

- Workload and immutable version registration through a documented API.
- A durable read-only research workflow over a fixed local corpus, invoked through MCP tools.
- Authentication, owner-private workload/run access and operator visibility.
- Execution history, intermediate step state, cited evidence and inspectable errors.
- Safe retries, timeouts, cancellation and failure inspection; controlled reruns are distinguished from replay.
- A local startup and usage guide that demonstrates the API and research workflow.

Later releases: local self-service environment provisioning and resource policies; quotas/rate limits; audit and metering; cloud Terraform deployment; Argo CD/GitOps and canaries; automatic release rollback; full failed-step replay tooling; multiple organizations/advanced tenancy; broader workload families; and the full portfolio/operational evidence package. Basic workload version registration remains in scope; runtime version rollout and progressive delivery are deferred. The full backlog is retained, not discarded.

This boundary is approved by the owner in D07. Feasibility and estimates must be rechecked after the F02 architecture decisions; five project hours/week is the accepted planning constraint. The first-release demonstration does not require implementing the later provisioning or operations journeys below.

## 5. Developer journey

1. Obtain an authorized identity and the relevant team/project context; the exact onboarding and boundary model is pending.
2. Generate or follow a minimal workload template.
3. Submit a versioned workload specification with tools, limits, permissions, and policy requirements.
4. For the first release, use the locally started runtime described by the quickstart. In a later release, request an execution environment and inspect asynchronous provisioning status.
5. Submit the selected representative workflow request.
6. Inspect execution progress, tool results, evidence, usage, and any uncertainty or failure.
7. Cancel an execution when necessary, or investigate a failure through the documented safe replay/rerun path.
8. Register a new workload version through the API. Runtime deployment and rollout inspection are later-release capabilities.

The first representative workflow is a **read-only research agent using MCP tools and producing cited evidence**, approved in D01.

Its concrete journey is to submit a research question and permitted source scope, receive an execution ID, observe source retrieval and intermediate steps, and retrieve a report whose factual claims link to supporting source evidence. On insufficient evidence, the result must expose that limitation rather than manufacture citations. A read-only tool failure or worker interruption must leave an inspectable execution that can recover according to the documented retry rules.

D06 approves a **fixed local document corpus through a read-only MCP tool**. Public web retrieval is outside the first demo. A representative request asks a question answerable from that corpus; the result cites document and passage references and exposes missing or conflicting evidence. Corpus fixtures should include an answerable question, an unanswerable question and conflicting material.

Exact corpus contents, input/output schemas, evidence storage and tool selection remain design decisions for later consultation. Retrieved document content is data and cannot expand the workload's tool permissions.

## 6. Operator journey

1. Deploy the platform from the documented local or selected cloud path.
2. Configure the approved identity, boundary, policy and capacity model.
3. Make the developer contract and supported templates available.
4. Observe provisioning outcomes, workflow behavior and attributable usage.
5. Investigate rejected requests or failures using correlated execution and audit history.
6. Roll out a platform version, detect representative failures, and invoke the documented rollback procedure.
7. Perform a backup/restore drill and assess recovery against the agreed objectives.

This is the longer-term operator journey. First-release operator work is limited to locally starting the API/workflow stack, configuring the approved access model and inspecting all owners' runs. Infrastructure provider, policy engine, deployment strategy and backup mechanism are decisions for later consultation.

## 7. Quality and trust requirements

- **Correctness:** validate inputs, preserve state intentionally, define ownership, and make create/reconcile/retry behavior idempotent where required.
- **Durability:** explain what survives API, controller, worker and dependency failures. Persistent execution history alone is insufficient evidence of safe side effects.
- **Security:** enforce the chosen identity and tenant/project model at API, provisioning and tool boundaries. Demonstrate negative cases and record limits rather than claiming production-grade security.
- **Observability:** correlate requests, provisioning operations and executions; expose useful duration, outcome, retry and resource measurements.
- **Developer experience:** provide clear status, errors, examples and a clean-environment quickstart. Record manual steps removed and remaining friction.
- **Operability:** test representative failures, deployment rollback and restore; document incident procedures and cost/capacity assumptions.
- **Maintainability:** version contracts, test behavior at meaningful boundaries, document architectural decisions, and keep examples and docs consistent with the implementation.

Numerical SLOs, retention periods, concurrency targets, budget and recovery objectives remain open. D08 approves establishing the F08 baseline before accepting performance thresholds; architecture and retention choices require later consultation. Quota, audit, metering, rollout and restore requirements apply to the later phases where those capabilities are implemented.

## 8. Success measures

| Measure | Evidence to collect | Target status |
|---|---|---|
| Environment setup improvement | Repeated manual baseline versus self-service provisioning, with equivalent prerequisites | Numerical reduction target pending |
| Initial native API baseline (D21) | Five retained-data startups, ten authenticated registration/read journeys and occupied-port detection/recovery; raw timings and conditions retained | Descriptive observations; targets pending owner review |
| Developer effort | Number and nature of manual steps removed; clean-onboarding observations | Target pending |
| Provisioning reliability | Successful/failed reconciliations, duplicate-resource checks, convergence and recovery duration | Scenario set and threshold pending |
| Durable execution behavior | Research success, transient tool failure, worker loss, timeout, cancellation, insufficient/conflicting evidence, and controlled rerun results | Acceptance procedure approved in D08; full replay follows later scope |
| Isolation and authorization | Two workload owners attempt authorized own-record and denied other-owner reads/writes; operator visibility is tested separately | Owner-private model accepted; exact action matrix requires later consultation |
| Workload breadth | Actual distinct test workloads successfully onboarded | Count and examples pending |
| Usage and cost | Attributable measured usage and dated per-workload/request cost assumptions | Budget/target pending |
| Adoption and communication | Developer/reviewer trials, feedback, documentation and demo usability | Intended trial audience pending |

Never replace these pending targets with unmeasured claims. Preserve raw results, environment details, workload definitions, dates, and known measurement limitations.

### Accepted acceptance procedure — D08

1. Run the fixed-corpus research workflow for an answerable question, an unanswerable question and a conflicting-evidence question. Verify cited document/passage references resolve and the result communicates evidence limitations.
2. Inject transient tool failure, worker interruption, timeout and cancellation. Verify bounded retry rules, inspectable attempts and terminal state, and persistence across restart. Verify duplicate submission using the same idempotency key does not create duplicate logical runs.
3. With two workload owners, test allowed own-record operations and denied other-owner reads/writes across workloads, runs and evidence. Separately test operator visibility and document its permissions. Apply the same tests to usage when metering is implemented later.
4. Have the owner review the F08 baseline and select numerical speed/capacity/recovery targets before subsequent benchmarks claim to meet them. Record test conditions and targets in the brief.

For the later provisioning phase, retain the approved comparison of **10 manual and 10 self-service provisioning measurements** under comparable prerequisites. Report timing distribution, steps removed and failure/recovery behavior. Do not count cluster/bootstrap time on one side while excluding it from the other. Quota/rate-limit denial tests belong to their later implementation phase. These deferred criteria do not expand the API-and-workflow first release.

The owner is responsible for deferred target decisions. Test workloads, hardware, load, recovery timing origin and baseline method must be consulted on and documented before measurements are interpreted as acceptance evidence.

## 9. Boundaries inherited from the supplied plan

- Compose established infrastructure products; do not recreate Kubernetes, Temporal, Backstage, or a model-serving layer.
- Keep the UI minimal. A generic dashboard or polished chatbot is not the primary product deliverable.
- Prefer coherent integration over superficially adding multiple orchestration frameworks.
- Require a demonstrated need before adding Redis or another queue.
- Use one selected model backend and one selected cloud deployment target initially; selection remains open.
- Document the threat model, tested properties, incomplete controls, and additional requirements before production use.
- Publish evidence-backed technical and career artifacts; do not claim scale, security or adoption that has not been demonstrated.

F02 records architecture decisions and alternatives in [architecture.md](architecture.md). The owner selected OpenRouter and local Kubernetes on 5 October 2026, superseding the original managed-cloud selection requirement; the testing model is `google/gemma-4-26b-a4b-it:free`, with Docker Compose first and kind later. Keycloak OIDC is included from the first release. This brief identifies user-facing requirements without choosing implementation mechanisms prematurely.

On 6 October 2026, D20 superseded Compose-first startup for F07: the owner requested no containers and accepted native PostgreSQL, Keycloak and Go API services inside Ubuntu WSL. The [native stack](local-stack.md) is now the first-release local startup path; later Kubernetes work remains outside this ticket.

## 10. Delivery and capacity

The imported project proposes 26 weeks from 5 October 2026 to 4 April 2027, originally covering API foundation, self-service provisioning, durable workflows, tenant security, operations, and evidence/launch. At the newly accepted capacity, that full-scope schedule is superseded by a smaller first release within the same six-month horizon; remaining capabilities move to a later roadmap.

The accepted capacity is **10 total hours/week, including 5 project hours and 5 hours for study/interviews/job search**, approved in D03. The original backlog estimates 193 project hours, whereas 26 weeks at five project hours provides 130 hours. D04 approves a smaller first release, and D07 selects the API and durable workflow only. Review/integration contingency must fit inside those 130 hours; revised estimates and scheduling follow the approved product scope and architecture.

The existing Wekan dates and capacity reference cards have not yet been rescheduled. They require revision once the first-release scope is accepted; the unchanged board dates must not be mistaken for the updated commitment.

Study, interview practice and applications run alongside the build. The product's implementation deliverables are distinct from those parallel career activities.

## 11. Risks and open questions

| Risk or question | Required action |
|---|---|
| Breadth exceeds part-time capacity | Agree the first-release cut and review estimates against actual progress |
| Research tool/evidence details remain unspecified | Consult on exact local-corpus tool/evidence semantics after F01 |
| Access enforcement could differ across endpoints | Design and test the accepted owner-private policy consistently; consult on exact actions |
| Overlapping state ownership across control plane, Kubernetes and workflow engine | Record responsibility and source-of-truth choices in F02 |
| Retries or reruns duplicate external effects | Define and test tool-side idempotency, authorization and compensation semantics |
| Demo security is mistaken for production readiness | Publish explicit trust boundaries, negative tests, gaps and production requirements |
| Benchmarks lack a valid baseline | Capture the manual baseline and reproducible test conditions before claiming improvement |
| Cloud dependencies or model access introduce cost and setup friction | Consult on backend choice and budget; keep a documented local path |

## 12. F01 completion checklist

- [x] Internal team request and problem statement reviewed by the owner.
- [x] Platform operator and application developer personas and journeys accepted.
- [x] All twelve capabilities mapped to observable acceptance evidence and distinguished from the first-release cut.
- [x] D01 first workflow decided and its concrete journey documented.
- [x] D02 initial customer shape decided; detailed access policy is tracked in D05.
- [x] D03 capacity assumption decided.
- [x] D04 smaller six-month release, D05 owner-private access and D06 local research corpus decided.
- [x] D07 concrete release cut and D08 acceptance procedure accepted.
- [x] First-release cut accepted; numerical thresholds deferred to owner review after F08.
- [x] Requirements separated from unapproved architecture choices and unmeasured claims.
- [x] Brief reviewed by the owner with “looks good” on 5 October 2026; F02 authorized.

No item is marked complete solely because this draft exists.
