# F27 verification — 12 October 2026

F27 exposes start, cancel, history, and result on the authenticated API. [D42](decision-log.md#d42--f27-execution-history-api) records the boundary. The [history guide](execution-history.md) lists the routes and the fields that stay hidden.

Checked with Go 1.27.1 against a disposable PostgreSQL database:

```text
go test -count=1 ./internal/execution ./internal/httpapi ./cmd/api
```

`TestSanitizeHistoryOmitsPromptsCorpusAndTokens` builds Temporal events whose payloads and failure text contain a bearer token, the question, and a corpus sentence. The public events are workflow start, retrieval start at attempt 2, and `retry_scheduled` with code `tool_unavailable`. The encoded events do not contain those strings. `TestPublicResultDropsUnsafeReasonAndNonIdentifiers` drops a prompt-like reason and a non-UUID evidence id.

`TestRunHistoryAndResultHTTP` starts a run with an idempotency key, repeats that key, and conflicts a changed question. A developer cannot read another owner's run. An operator can read and cannot cancel that run. Cancel is 202 twice for one command. History returns steps, attempts, timestamps, and a safe failure, including a failure message that is replaced when it contains a bearer token. A result before completion is 409. A completed result returns the decision, outcome, and evidence id without the question. With no Temporal reader, history and result are 503 and the run read stays available with `execution.source` `unavailable`.

The health-only process still returns 404 for `POST /api/v1/runs`. Product mode is required.

## Material limits

- Authentication is the existing Keycloak verifier: issuer, audience, signature, expiry, and the developer and operator roles. Full tenant RBAC is the M4 security gate and is not implemented here.
- No live Temporal dev server was started. The HTTP test injects a reader. `TEMPORAL_ADDRESS` is unset in the native stack, so a normal API process returns 503 for history and result.
- There is no product report or evidence table. `GET /report` and `GET /evidence` are not implemented. The result does not include claim text or corpus excerpts.
- Run list and rerun are not implemented.
- The run resource includes the submitted question because the accepted Run schema requires it. History and result omit it.
- A canceled result does not include the compensation note. That note remains in Temporal history.
- No execution-status column was added.
