# Registration profiling procedure

F10 uses an opt-in test harness around the real registration handler, lifecycle server, PostgreSQL repository and signed OIDC/JWKS fixture. Each client repeatedly creates a workload, creates its immutable version, reads the workload and reads the version over real loopback HTTP. One client level runs first, then two. Each uses five seconds of warmup and fifteen seconds of measurement, with no race instrumentation. The generator stops starting journeys at the phase deadline and drains journeys already started; actual durations include that drain.

This is a closed-loop observation: slow responses reduce the offered request rate. The observed completion rate is not a capacity ceiling. There is one short sample per level, no production SLO, and no statistical claim about sustained traffic. D23 accepts a [5 ms request-p95 local repeat-test limit](performance-targets.md), separately at one/two clients. Do not compare these numbers directly with F08's resource-capped native process and real Keycloak journey.

Use an isolated native PostgreSQL administrator DSN supplied privately in `FORGE_TEST_ADMIN_DATABASE_URL` and set `FORGE_REQUIRE_DB_TESTS=1`. The fixture creates random disposable databases and migration/runtime roles, applies real migrations, grants runtime permissions and removes only its own database/roles afterward. The fixture pool allows twelve connections. The API operation deadline is five seconds and the client deadline ten seconds. Never point a profile at the retained Forge product database or use real corpus content/tokens. No containers or public profiling endpoint are needed.

From a clean checkout in a Linux shell with Go 1.27.1, Python 3 and those database variables already set:

```sh
export FORGE_PROFILE_COMMIT=$(git rev-parse HEAD)
# Choose a new absolute directory; the harness refuses to overwrite an existing one.
export FORGE_PROFILE_OUTPUT=/tmp/forge-f10-new-observation
go test -count=1 -timeout 120s -v -run '^TestRegistrationLoadProfile$' ./internal/service
python3 scripts/profile-summary.py "$FORGE_PROFILE_OUTPUT" > "$FORGE_PROFILE_OUTPUT/summary.json"
python3 scripts/check-performance-targets.py --profile-summary "$FORGE_PROFILE_OUTPUT/summary.json"
go tool pprof -top "$FORGE_PROFILE_OUTPUT/clients-1/cpu.pprof"
go tool pprof -top -tagfocus=component=api "$FORGE_PROFILE_OUTPUT/clients-1/cpu.pprof"
go tool pprof -top -sample_index=inuse_space "$FORGE_PROFILE_OUTPUT/clients-1/heap-after.pprof"
go tool pprof -top -sample_index=alloc_space -base "$FORGE_PROFILE_OUTPUT/clients-1/heap-before.pprof" "$FORGE_PROFILE_OUTPUT/clients-1/heap-after.pprof"
```

Repeat pprof inspection for `clients-2`. CPU sampling covers the measurement phase. Heap snapshots follow explicit garbage collection before and after measurement; heap in-use describes sampled retained allocations, and the allocation-space difference describes sampled allocation churn. MemStats records exact combined-process heap and total-allocation deltas. These profiles include the API, HTTP client, signed issuer fixture, JSONL recorder and Go runtime; PostgreSQL runs in a separate process and its CPU/memory is excluded. CPU handler labels help separate API stacks but do not turn heap data into API-only memory measurements. [Go's profiling documentation](https://pkg.go.dev/runtime/pprof) describes these profile types and inspection tools.

Each client directory preserves CPU and both heap profiles, `conditions.json` and `requests.jsonl`. Every request records its phase, operation, status or transport failure, latency, client and journey number. Journey completion records include failed attempts. Preflight/warmup remain in raw output and are excluded from measured distributions. Failures and outliers remain in measured distributions. Summaries use nearest-rank p50/p95/p99, per-operation distributions, explicit request/journey error denominators, status counts and actual elapsed time. Raw JSONL may be compressed losslessly as `requests.jsonl.gz`; the summary script accepts either form. The owner selected public code/report publication with raw records/profiles kept local; `docs/profiles/` is ignored accordingly.

`FORGE_PROFILE_SMOKE=1` reduces each level to 100 ms warmup/300 ms measurement for CI pipeline verification. CI asserts two populated levels and zero observed request/journey failures. Smoke results are not PC performance evidence. Ordinary tests skip profiling; setting the output variable is the explicit opt-in. Output directories/files start with private permissions and contain synthetic traffic, no DSNs, authorization headers, request/response bodies or personal documents. Check contents before publishing; profile symbol data can include local source paths.
