# Failure injection — F47

[D62](decision-log.md#d62--f47-failure-injection) records the boundary. The raw sample is [f47-failure-injection.json](validation/f47-failure-injection.json).

## Command

The suite is one Go test. Database and quota scenarios run when `FORGE_TEST_ADMIN_DATABASE_URL` points at a disposable PostgreSQL. Set `FORGE_FAILURE_OUTPUT` to write a new raw JSON file.

```text
go test -count=1 -timeout 180s -run TestFailureInjection ./internal/failure
```

## Scenarios

| Scenario | What is injected | Recovery |
|---|---|---|
| `worker_loss` | The worker is replaced by a replay of constructed history. | Activity entries and tool calls stay at zero. |
| `tool_model_outage` | A denied document id, one tool error, then a fixture model error. | The denied call starts no tool. The tool error is redacted. One evidence row and one report are stored. |
| `bad_rollout` | 40 errors in 100 requests and 800 ms latency. | The staging pin returns to `local-overlay`. A second call with no regression does not write it again. Execution identifiers stay. |
| `database_outage` | The disposable role loses login and its sessions are terminated. | `/healthz` stays 200. The workload read returns 503, then 200 for the owner. Another subject still receives 404. The row count stays one. |
| `quota_exhaustion` | The tenant workload cap is set to one. | The second create returns the quota error. The other subject cannot read the first workload. The row count stays one. |

## Limits

No `forge-worker` process was killed. No Temporal dev server was started. Replay uses constructed history. The tool and model are in-process fixtures, not OpenRouter and not an MCP process. Rollback rewrites a pin file and does not change a live cluster, reverse a migration, or unsend a model call. The database outage uses a disposable role, not the retained product database. The quota cap is one process, not a distributed quota, and `small-v1` is unchanged.
