# Product and design decision log

Owner: ahmad

Status: D01–D08 accepted on 4 October 2026. Detailed architecture and baseline thresholds require later consultation. Approval is recorded only for explicit owner responses.

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

## Decisions to consult on after F01

- Quantitative success thresholds and test conditions after the agreed baseline milestone.
- Exact operator/developer action matrix within the approved owner-private access model; future side-effecting tools require a separate decision.
- Architecture choices in F02: state ownership, API contracts, workflow engine, policy engine, model backend, isolation mechanism, cloud target, and delivery strategy.
- Retention periods, deployment exposure, cloud budget, and operational recovery objectives.

## Decision process

For each material choice, record the question, options and tradeoffs, the owner's response, and the resulting change to requirements or acceptance criteria. Do not treat silence as approval. Distinguish requirements taken directly from the supplied plan from proposed design choices.
