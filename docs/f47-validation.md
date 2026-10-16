# F47 verification — 16 October 2026

F47 runs five failure injections and checks recovery, isolation, and duplicate side effects. [D62](decision-log.md#d62--f47-failure-injection) records the boundary. The command and limits are in [failure-injection.md](failure-injection.md). The raw sample is [f47-failure-injection.json](validation/f47-failure-injection.json).

Checked with Go 1.27.1. Database and quota used disposable PostgreSQL 18.6. The retained product database was not opened.

```text
go test -count=1 -timeout 180s -run TestFailureInjection ./internal/failure
```

| Scenario | Duration | Side effects |
|---|---|---|
| Worker loss | 38118 µs | 0. Replay did not call the tool. |
| Tool and model outage | 30695 µs | 1 evidence row. The secret-shaped tool error was redacted. |
| Bad rollout | 33 µs | 1 pin write. The second call did not write again. |
| Database outage | 17329 µs | 1 workload row. Owner read recovered. The other subject stayed at 404. |
| Quota exhaustion | 16572 µs | 1 workload row. The second create was limited. |

No worker process was killed. No Temporal server, OpenRouter call, or MCP process was started.
