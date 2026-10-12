# Execution history and result — F27

Developers can start a run, cancel their own run, and read execution state, history, and a terminal result. [D42](decision-log.md#d42--f27-execution-history-api) records the boundary. PostgreSQL still stores the run and the start or cancel command. Temporal still stores execution history. This API does not add an execution-status column.

## Routes

| Operation | Method and path | Success |
|---|---|---|
| Start | `POST /api/v1/runs` | 202 after the run and pending start command commit. Requires `Idempotency-Key`. |
| Read | `GET /api/v1/runs/{run_id}` | 200 with submission, cancellation, and a Temporal observation |
| Cancel | `POST /api/v1/runs/{run_id}/cancel` | 202 and one logical cancel command. No body. |
| History | `GET /api/v1/runs/{run_id}/history` | 200, steps ordered by sequence |
| Result | `GET /api/v1/runs/{run_id}/result` | 200 when Temporal has a terminal value |

The same Keycloak access-token verifier used by registration authenticates every route. A developer reads and cancels only their own runs. An operator can read every owner's run and cannot cancel or start a run for a workload they do not own. Another owner's id is 404. Full tenant RBAC is the M4 security gate. This ticket does not add it.

Errors keep the existing shape: `{error:{code,message},request_id}` and `X-Request-ID`.

## History

Each event has `event_id`, `sequence`, `step`, `attempt`, `event_type`, `occurred_at`, and `failure`. `failure` is null or `{code,message}`. The message is a fixed sentence for a known class such as `tool_unavailable`. Temporal failure text is not copied.

| Activity | Public step |
|---|---|
| Workflow lifecycle | `workflow` |
| `Retrieve` | `retrieval` |
| `Generate` | `generation` |
| `RecordTerminal` | `persistence` |

`Compensate` is omitted. The public step list has no compensation value. Citation checking stays inside generation, so there is no separate `citation_validation` event. Activity inputs and outputs are not returned. That removes the question, corpus excerpts, and bearer tokens from history.

An absent workflow returns an empty page. A Temporal error, or an API process with no Temporal client, returns 503. The page does not invent events. Cursors are the existing HMAC cursors, bound to the caller and the run, ordered by sequence.

## Result

`GET /result` returns `run_id`, `decision`, `reason`, an optional `outcome`, and `evidence_ids`. Decisions are `complete`, `fail_closed`, `canceled`, or `timed_out`. Evidence ids are lowercase UUID version 4 values. A reason that is not one of the known classes is replaced. A running or absent workflow returns 409. A missing Temporal client returns 503.

The OpenAPI `Report` and `Evidence` schemas require claim text and excerpts. Those values are not in a product table, and this API does not return them. `GET /report` and `GET /evidence` stay closed. `GET /api/v1/runs` list and `POST /rerun` stay closed. Rerun is a later ticket.

The run read still includes the question and document ids because the accepted `Run` schema requires them. History and result do not repeat that question.

## Temporal client

The API dials Temporal only when `TEMPORAL_ADDRESS` is set. `TEMPORAL_NAMESPACE` defaults to `default`. The native stack does not set the address and does not start `scripts/temporal-dev.ps1`. Without a client, start and cancel still commit commands, the run read reports `execution.source` `unavailable`, and history and result return 503.

A canceled workflow's public result is decision `canceled` and reason `canceled`. The compensation note remains in Temporal history. This read does not copy it out of the workflow value when the client does not return that value. Raw history is capped at 4096 events; a longer history returns 503 rather than a truncated page.
