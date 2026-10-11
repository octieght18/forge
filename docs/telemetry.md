# Telemetry — F17

Forge records provisioning and reconciliation measurements with OpenTelemetry in the process that produced them. The view is a JSON document of outcome counts, durations, bounded retry and failure reasons, and recent spans. There is no collector, Prometheus scrape port, or dashboard. [D31](decision-log.md#d31--f17-in-process-opentelemetry-views) records that boundary. The controller-runtime metrics server stays disabled.

## Operator view

`GET /api/v1/telemetry` returns the API process view. An operator token is required. A developer receives 403. The document counts terminal provisioning results (`ready`, `failed`, `canceled`, `timed_out`) and the seconds from creation until that result. It also lists the latest spans, each with `trace_id`, `request_id`, and `operation_id` when those identities are safe lowercase IDs.

```powershell
$view = Invoke-RestMethod -Headers @{ Authorization = "Bearer $token" } -Uri http://127.0.0.1:8081/api/v1/telemetry
$view.provisioning.outcomes
$view.spans
```

`$token` is the operator access token from the private login file. Do not put it in a command that is logged. This view does not include another process's counters. Reconciliation retry reasons are recorded in the environment controller process, which does not publish a Service or scrape endpoint.

## What is measured

| Event | Count | Duration | Reason |
|---|---|---|---|
| Provisioning reaches `ready`, `failed`, `canceled`, or `timed_out` | One outcome | Creation until the terminal update, when that interval is positive and under 15 minutes | The outcome is the result. Controller error text is not a metric label. |
| Controller pass ends `Ready` or `Absent` | `success` | The pass, when it is under one minute | Not labeled. |
| Controller pass ends `Provisioning` or `Deleting` | `retry` | The pass | A fixed reason such as `NamespacePending`, `CleanupPending`, or `OwnershipConflict`. Any other text is `unspecified`. |
| Controller pass ends `Failed` or returns an error | `failure` | The pass | The same fixed vocabulary. An error without a reason is `reconcile_error`. The error string is not stored. |

A pass that does not reach one of those phases is `idle`. A duration the process clock reports as zero is omitted.

## Traces

Each API request continues an incoming W3C `traceparent` and returns the same trace ID on the response. The span is `forge.http`. Its attributes are the server `request_id` and, for an operation route, the `operation_id`. A terminal provisioning update adds a child span named `forge.provisioning` with both identities. Tokens, owner subjects, prompts, and corpus text are not span attributes. The view keeps the latest 32 spans.

Controller passes start their own traces. They do not receive the API request context, so a live reconcile span does not inherit the caller's request ID. The API span still links the request and the operation when the caller accepts, reads, or cancels that operation.

See [F17 verification](f17-validation.md) and [provisioning operations](provisioning-operations.md).
