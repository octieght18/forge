# F11 — Research specifications and lifecycle

Accepted on 8 October 2026 in two owner replies. F11 is adapted to the API/durable-workflow first release. The original environment resources, images, deployment policy and Kubernetes lifecycle remain deferred. [D24](decision-log.md#d24--f11-research-specification-and-lifecycle) records the amendment. [OpenAPI](../internal/contract/openapi.json) is the executable payload authority; this guide defines who may change each state and what an observation means.

## Desired research work

| Record | Declaration and authority |
|---|---|
| Workload | Owner-private name/description. The authenticated issuer/subject owns it; the client cannot set or transfer ownership. Metadata PATCH requires the current revision/ETag. |
| Version | Immutable `research.v1` spec, server-issued ID and per-workload version number, semantic SHA-256 fingerprint. New desired research behavior requires another version. |
| Run | Exact workload/version, question and explicit document subset frozen on acceptance. The version does not change when workload metadata changes. |

The [version example](../internal/contract/examples/create-version.json) declares an immutable corpus snapshot digest; exactly `corpus.search` and `corpus.read`; 1–64 unique approved document IDs; 1–16 passages; 128–2,048 output tokens; `research-report.v1`; and the accepted explicit free model slug. The [run example](../internal/contract/examples/create-run.json) requires UUID v4 resource IDs, a nonblank question of at most 4,000 characters and 1–64 unique document IDs within the version's scope. Mutation bodies remain bounded at 64 KiB.

The operator approves snapshots/documents during registration. The future worker must enforce the frozen scope again before retrieval. A valid declaration or matching fingerprint grants no permission. The worker owns the configured read-only MCP process and fixed retrieval/generation stages; users cannot supply executable commands, tools, filesystem paths, arbitrary URLs or model-selected extra capabilities. [Architecture](architecture.md) defines model routing and artifact boundaries. Schema ceilings are research limits, not CPU/memory reservations or a tenant quota system.

Images, replicas, CPU/memory requests, deployments, desired environment state and write/network permissions are unsupported declaration fields and are rejected. Platform binary versions, native systemd resource caps and model routing policy remain operator configuration outside the immutable research spec.

## Three separate state authorities

| Axis | Values | Meaning and writer |
|---|---|---|
| Submission | `pending`, `dispatched` | PostgreSQL start intent is durable; future dispatcher marks acknowledgement only after confirming the exact Temporal workflow identity. |
| Cancellation | `not_requested`, `requested`, `delivered` | No cancel command; durable pending cancel command; acknowledged cancellation request. The owner requests it, and the dispatcher records delivery. |
| Execution | `not_started`; Temporal `running`, `succeeded`, `failed`, `canceled`, `timed_out`, `terminated`; or `unknown` | Execution state is obtained from Temporal. The API can expose an unavailable observation with an optional timestamped cache; it cannot infer completion from commands or artifacts. |

PostgreSQL command `pending` maps to submission `pending` for start and cancellation `requested` for cancel. Command `delivered` maps to `dispatched` or cancellation `delivered`, respectively. No cancel row maps to `not_requested`. Command rows contain their own created/delivered timestamps; `execution.observed_at` belongs to the Temporal observation, not command delivery.

## Transition and acknowledgement rules

| Trigger | Durable/product transition | Execution consequence |
|---|---|---|
| First valid submission | Atomically commit run plus pending start command; then 202 is permitted | Acceptance alone proves no execution outcome. |
| Matching owner/idempotency-key retry | Return the existing run; changed semantic input conflicts | Keep the same `forge-run/<run UUID>` workflow identity. |
| Start acknowledged or reconciled | Start command `pending → delivered`; submission `pending → dispatched` | Report an actual Temporal observation, or `unknown` when lookup is unavailable. |
| Ambiguous start response/crash before acknowledgement | Keep start command pending until identity is reconciled | Temporal may already be running or closed. A pending command does not prove `not_started`. |
| Owner requests cancel, including while start is pending | Insert one pending cancel command; `not_requested → requested`; repeats retain that intent | No claim that the workflow stopped. Start acknowledgement gates delivery. |
| Cancel request acknowledged | Cancel command `pending → delivered`; cancellation `requested → delivered` | Completion can win the race; `succeeded` remains a valid observed result. |
| Transient dispatch failure | Preserve pending command and original identity | Later dispatcher supplies bounded retries; a transport error is not workflow failure. |
| Confirmed terminal run is rerun | New run, new start command/identity, original frozen inputs and `rerun_of` link | Original terminal execution stays closed; rerun is distinct from deterministic replay. |

For one Temporal execution, running may become any listed terminal state; a poll may skip running and first observe a terminal state. Transient activity/Workflow Task errors do not independently justify `failed`. This table describes observations, not a Forge state machine controlling Temporal. Workflow retry/chains and terminal verification must use the appropriate Temporal identity when implemented; an older run's observation cannot overwrite a newer observation as current.

No command transitions backwards to pending after delivery. Logical delivery is idempotent, but network attempts and inference may repeat. A model response lost before artifact persistence may be generated again; no exactly-once inference claim is made.

Cancellation is cooperative, so acknowledgment is separate from the closing status. Termination is a different operation and has no Forge mutation endpoint in this release. See [Temporal cancellation](https://docs.temporal.io/encyclopedia/workflow/cancellation-and-termination). Stable Workflow IDs, conflict handling and rejection of closed-ID reuse must be explicit; Temporal's reuse checks only cover retained history. The later dispatcher must fail closed on an unresolved historical start rather than assume a missing retained execution never existed. See [Temporal ID policies and retention limitations](https://docs.temporal.io/workflow-execution/workflowid-runid).

## Observation consistency

The owner approved three executable safeguards:

- Cancellation `delivered` requires submission `dispatched`.
- Submission `dispatched` rejects execution source `not_started`.
- Unavailable execution pairs `last_known_state` with its original `observed_at`; both are null when there is no cached observation.

`source: temporal` requires a timestamp and a current observed state, with null `last_known_state`. `source: unavailable` always reports `state: unknown`, even if the last known state was succeeded. Do not replace the cache timestamp with the failed lookup time. `source: not_started` is permitted only when no start can have occurred; future dispatch code must establish that fact, not merely inspect a pending row.

Schema validation enforces payload consistency, including run pages, but cannot prove origin, observation freshness, command acknowledgements or monotonic cache updates. Readers must assemble coherent command state and enforce authorization before emitting a response. These are obligations for later run/status implementation; no polling freshness interval is selected here.

The [ambiguous-start example](../internal/contract/examples/ambiguous-start-run.json) permits pending submission with a Temporal running observation. The [completion-race example](../internal/contract/examples/cancel-completion-race-run.json) permits delivered cancellation with observed success. [Unavailable status](../internal/contract/examples/unavailable-run.json) retains the timestamp of the last observation while reporting current uncertainty.

## Ownership, revisions and deferred environment state

Developers read/mutate their own resources. Operators inspect all owners but still mutate only their own. Apply this to cancel/rerun, nested history/evidence/report reads and collections. Missing and inaccessible IDs both return 404. Authentication supplies exact issuer/subject; workload metadata revision/ETag handles concurrent edits. Version number and fingerprint identify immutable declarations, and the run freezes one exact version.

These values do not implement Kubernetes `generation`, `observedGeneration` or conditions. A future environment resource would have a distinct desired specification and controller-observed state. The original Pending/Provisioning/Ready/Failed/Deleting lifecycle, resources, image policy, reconciliation generations and deletion/cancellation rules require a later design decision before CRD/controller work. Do not alias them to research runs: a ready environment is not a successful research execution. Native API readiness indicates service health, not a workload's execution state.

## Delivery boundary

F11 delivers this lifecycle contract, strengthened OpenAPI consistency checks, examples and regression tests. Workload/version registration already runs; repository run/start/cancel acceptance already exists. Public run/status/cancel/rerun endpoints, command delivery, Temporal workers, cache storage, MCP/model execution and artifacts remain later tickets. No database migration, extra dependency, container, Kubernetes resource or model call is introduced. See [F11 verification](f11-validation.md).
