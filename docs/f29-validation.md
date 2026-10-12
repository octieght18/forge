# F29 verification — 12 October 2026

F29 measures what the SDK test suite does after a half-finished run, a dependency failure, an uncertain result, and a cancellation. [D44](decision-log.md#d44--f29-durable-recovery-measurements) records the boundary.

Checked with Go 1.27.1:

```text
go test -count=1 -v -run TestDurableRecoveryMeasurements ./internal/execution
go test -count=1 ./internal/execution
```

`TestDurableRecoveryMeasurements` is one sample on this VM. It is not a production SLO. The Temporal CLI was not installed, and `scripts/temporal-dev.ps1` was not started. No worker process was killed.

## Measured sample

| Case | Wall time | Attempts and result |
|---|---|---|
| Dependency failure after retrieval started | 43361 µs | `Retrieve` 2, `Generate` 1, `Compensate` 1, `RecordTerminal` 1. Tool calls 2. Evidence rows 1, retention `compensated`. |
| Uncertain result | 12993 µs | `Retrieve` 1, `Generate` 1, `Compensate` 0, `RecordTerminal` 1. Outcome `insufficient_evidence`. Evidence rows 0. |
| Cancellation during retrieval | 1816 µs | Retrieval was started and the activity was canceled. Terminal state `canceled`. |
| Replayer after a recorded history | 535 µs | Activity entries 0. The new replayer did not call the tool or the model. |

The suite log for the dependency failure said `TimeSkipped 1s` on the one-second retry timer. The 43361 µs figure is wall time. The test clock did not sleep for that second.

## What was not run

A live kill of `cmd/forge-worker` and a resume against a Temporal dev server was not run. The worker restart in this ticket is the replayer reading the same history. Attempt counts above come from the SDK activity listener, not from a server history download.
