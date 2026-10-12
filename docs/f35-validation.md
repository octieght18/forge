# F35 verification — 12 October 2026

F35 adds an API request rate and a tenant cap on workloads and pending runs. [D50](decision-log.md#d50--f35-quotas-and-rate-limits) records the boundary. The namespace profile `small-v1` is unchanged and still caps one namespace.

The default request window is 200 requests per second per tenant inside one API process. The default caps are 100 workloads and 100 pending start commands per tenant and project. A limited response is HTTP 429, `Retry-After: 1`, and `{"error":{"code":"rate_limited","message":"Retry later"},"request_id":...}`. It does not include a stack trace.

Checked with Go 1.27.1 and a disposable PostgreSQL database:

```text
go test -count=1 ./internal/httpapi ./internal/store
```

`TestSingleProcessQuota` uses a window of 2 and caps of 2 workloads and 1 pending run. The third read in that window is 429. Eight concurrent creates produce 2 rows and 6 limited responses. A second pending run is 429. The workload count uses a PostgreSQL transaction advisory lock, so concurrent callers in that database do not pass the cap. The request window is a mutex in the API process. Another process would keep its own window. There is no distributed quota consensus.

These numbers are local admission limits, not a capacity SLO.
