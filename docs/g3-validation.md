# G3 verification — Durable AI execution and safe replay

Reviewed on 11 November 2026 against commit `ad79673`. [D78](decision-log.md#d78--g3-durable-execution-and-safe-replay) records the boundary. This is an owner evidence review, not an automatic date gate.

## Checklist

| Acceptance | Result | Evidence |
|---|---|---|
| Transient failure, partial completion, timeout, cancellation and uncertain results have defined behavior | **PASS** | SDK `TestDurableRecoveryMeasurements` (this session) + live cancel accepted; see samples below |
| Replay/re-execution distinctions and side-effect safety are demonstrated | **PASS** | Worker-down pending → same run succeeded after worker restart; live `POST .../rerun` created a new run with `rerun_of`; [replay-modes.md](replay-modes.md) |
| Real workflow resolves context, calls authorized tools, retains intermediate state and produces evidence | **PASS** | Live Temporal history (retrieval/generation/persistence) and rerun result `complete` / `answered` with evidence id |

## Offline suite (this session)

```text
go test -count=1 -v -run TestDurableRecoveryMeasurements ./internal/execution
```

| Case | Wall time | Notes |
|---|---|---|
| Dependency failure (half) | 45731 µs | Retrieve×2, Generate×1, Compensate×1, evidence=1 |
| Uncertain result | 13123 µs | |
| Cancellation | 2170 µs | `cancel_state=canceled` |
| Replayer | 528 µs | |

One sample on this VM. Not a production SLO. Test clock skipped the 1s retry sleep (`TimeSkipped 1s`).

## Live Temporal stack

Processes used for the review:

- Temporal CLI `1.10.0` via `pwsh -File .\scripts\temporal-dev.ps1` (`127.0.0.1:7233`)
- `forge-dispatch` with `FORGE_DISPATCHER_DATABASE_URL` (dispatcher role) and `TEMPORAL_ADDRESS`
- `forge-worker` with `TEMPORAL_ADDRESS` and `FORGE_DEBUG=true` (in-process corpus/model fixtures)
- API on `http://127.0.0.1:8085` with `TEMPORAL_ADDRESS` (native systemd API on `8081` still has no Temporal client; history/result there stay unavailable)

Native Postgres/Keycloak remained on the local stack (`55436` / `8082`).

### Worker loss / resume

Fixture runs complete in ~100ms, so mid-activity Ctrl+C is unreliable. The demonstrated path:

1. Stop `forge-worker`.
2. Start runs while the worker is down → `submission_state=pending`, `execution.source=not_started` (examples: `a137ceaa-0aba-479d-9e68-b5c4f2f58d60`, `56e1d8c1-cb75-4184-996b-9f7961b2f83a`).
3. Restart worker with `FORGE_DEBUG=true`.
4. Same run ids become `dispatched` and Temporal `succeeded` without a second client start.

Also observed: `f8f5d899-da9c-46eb-afbe-6c33062146d4` and `a8e4b3c2-2726-46a0-a16b-e586a6d813d9` reached Temporal `succeeded` after worker restarts. History for `60372267-013e-4109-9cd5-f03edb883f18` showed workflow → retrieval → generation → persistence → workflow succeeded.

### Re-execution (not replay)

```text
POST /api/v1/runs/f8f5d899-da9c-46eb-afbe-6c33062146d4/rerun
```

- Child run: `9cd846f7-079d-4fdc-b6c0-c2414675cb95`
- `rerun_of`: `f8f5d899-da9c-46eb-afbe-6c33062146d4`
- Result: `decision=complete`, `reason=answered`, `outcome=answered`, evidence `e2e0b99d-2c50-418b-930f-6ec4b28f781e`

### Cancellation

`POST /api/v1/runs/a8e4b3c2-2726-46a0-a16b-e586a6d813d9/cancel` returned **202** with `cancellation_state=requested` after that run had already Temporal-succeeded. Mid-flight cancel was not separately captured in this session; SDK cancel sample remains the quantified cancel proof (`2170 µs`, `canceled`).

## Limits

- Worker uses `FORGE_DEBUG=true` fixtures, not live OpenRouter/MCP processes.
- API with Temporal was a second process on port **8085**; native unit on **8081** still omits `TEMPORAL_ADDRESS`.
- Workflow timeout 2m / activity 30s / heartbeat 10s remain demo bounds, not SLOs.
- Mid-activity process kill during Retrieve was not timed; proof is worker-down with pending commands then pickup of the **same** run ids.
- G4–G6 stay open.

## Related docs

- [workflow-replay-guide.md](workflow-replay-guide.md), [replay-modes.md](replay-modes.md)
- [f29-validation.md](f29-validation.md), [f47-validation.md](f47-validation.md), [failure-injection.md](failure-injection.md)
- [temporal-worker.md](temporal-worker.md), [execution-history.md](execution-history.md)
