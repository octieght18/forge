# Execution, tool, and evidence contract — F21

This contract defines a research run before a worker exists. [D35](decision-log.md#d35--f21-execution-tool-and-evidence-contracts) records the boundary. [OpenAPI](../internal/contract/openapi.json) validates the payloads. No run route, Temporal worker, or MCP process is started here. The G2 provisioning gate remains on the Backlog, so this design does not treat measured Kubernetes self-service as accepted.

The durable parts are the request, the workflow decisions, the activity results, and the evidence. Prompt text is an input to one activity. It is not the workflow.

## Request and execution

`CreateRunRequest` freezes the workload, version, question, and document subset. Acceptance inserts that request. It does not start retrieval or a model call.

`ExecutionStatus` is a separate observation:

| Source | What it means |
|---|---|
| `not_started` | The dispatcher has established that no start occurred |
| `temporal` | Temporal reported `running`, `succeeded`, `failed`, `canceled`, `timed_out`, or `terminated` |
| `unavailable` | The current state is `unknown`. A previous observation may remain in `last_known_state` with its original timestamp |

An unavailable lookup is an uncertain execution result. The API keeps the last observation and does not invent a terminal state.

## Deterministic decisions and activities

A `WorkflowDecision` is replayable. `deterministic` is true. The decision is one of `start_retrieval`, `start_generation`, `validate_citations`, `persist`, `complete`, or `fail_closed`. The record has a run ID and a sequence. It has no excerpt, model text, tool output, or clock reading. Replay rebuilds these choices from history.

An `ActivityResult` is the opposite. `deterministic` is false. The step is `retrieval`, `generation`, `citation_validation`, or `persistence`. Each attempt carries `input_sha256`, the fingerprint of that attempt's input.

| Outcome | Meaning |
|---|---|
| `succeeded` | The activity finished and its output was committed. `failure` is null |
| `reused` | A later attempt found the committed output for the same fingerprint and made no new model or tool call. `failure` is null |
| `failed` | The attempt stopped with an `ActivityFailure`. A class is required |

A crash after a model response is generated and before that output is committed is a new attempt, not `reused`. Replay of the workflow does not recover an unrecorded provider response.

## Tool permission and MCP invocation

The version permits exactly `corpus.search` and `corpus.read`. The run's `document_ids` are a subset of that version. The worker launches MCP from operator configuration over stdio. The workload does not supply the command, a path, a URL, or another tool. Model text cannot add a tool.

`MCPInvocation` is the call:

- `corpus.search` takes the frozen question as `query`, the snapshot, the frozen document IDs, and a limit of 1–16.
- `corpus.read` takes the snapshot, one document ID, and one passage ID that a search in this run already returned.

`MCPResult` returns at most 16 passages. Each passage has `document_id`, `passage_id`, `text_sha256`, and `excerpt`. An empty `passages` array is a successful search with nothing to cite. It is not `tool_denied`.

## Typed failures

`ActivityFailure.class` is one of:

| Class | When the worker uses it |
|---|---|
| `tool_denied` | The document or passage is outside the frozen scope |
| `tool_unavailable` | The MCP process cannot be reached |
| `tool_invalid` | The tool response is outside `MCPResult` |
| `model_unavailable` | The selected model route cannot be reached |
| `model_invalid` | The model output is outside the report schema |
| `citation_invalid` | A claim cites evidence that is not in this run |
| `persistence_conflict` | The committed output for this fingerprint does not match the new attempt |
| `cancelled` | Cancellation was applied to the attempt |
| `timed_out` | The attempt passed its deadline |

The message is at most 512 characters and carries no corpus text, token, or filesystem path. `insufficient_evidence` and `conflicting_evidence` are report outcomes, not failure classes. A finished report can be uncertain without failing the activity.

## Evidence provenance and uncertain results

`Evidence` is the provenance record: `evidence_id`, `run_id`, `corpus_snapshot_id`, `document_id`, `passage_id`, `text_sha256`, and `excerpt`. A claim lists those evidence IDs. The hash ties the excerpt to the snapshot passage. Matching hashes show the citation points at the retrieved text. They do not show that the passage supports the claim.

`Report.outcome` is `answered`, `insufficient_evidence`, or `conflicting_evidence`. `answered` requires at least one claim. The two uncertain outcomes require at least one limitation and may have no claims. The [uncertain report example](../internal/contract/examples/report-uncertain.json) is the second case.

## What this design leaves for later tickets

F22 starts Temporal workers. F23 implements context resolution and the MCP adapters. The worker now runs the decision sequence and stops at `model_unavailable`. Public run, history, evidence, and report routes stay unimplemented. The installed environment controller remains a namespace boundary and is not a research execution.
