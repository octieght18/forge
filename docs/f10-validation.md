# F10 API foundation validation and profiles

Completed implementation and local observations on 7 October 2026. The owner accepted real HTTP/database cancellation and drain tests, signed OIDC profiling fixtures, file-only profiles, and a reduced one/two-client protocol: five seconds warmup and fifteen seconds measurement per level. [D22](decision-log.md#d22--f10-cancellation-and-reduced-registration-profiling) records that consultation and the separately approved correction to body-read ordering. [Procedure](profile-procedure.md) gives reproducible commands and interpretation limits.

## Cancellation, persistence and observed correction

| Case | Evidence/assertions |
|---|---|
| Client disconnect during PostgreSQL update | Real authenticated PATCH reaches an AFTER UPDATE trigger blocked on an advisory lock. Client cancellation reaches the handler, removes the blocked backend and releases the pool acquisition. Revision/description remain `1/original`; an HTTP read succeeds afterward. This proves rollback of an executed mutation. |
| Graceful drain across PostgreSQL | Stop serving while the update is blocked; readiness becomes false and the dependency remains active. Release its lock within grace: HTTP 200, revision `2/changed`, no premature request cancellation, clean server exit. |
| Expired shutdown grace | Leave the update blocked through the shortened test grace. Server reports deadline expiry, cancels/closes the request, releases the blocked backend/pool connection and preserves revision `1/original`. A subsequent SQL read succeeds. The client may see connection close or a raced 503. |
| OIDC discovery cancellation | Cancel startup after the real discovery handler starts; discovery returns failure and its upstream HTTP request is canceled. |
| JWKS caller cancellation | Cancel a caller after the key endpoint starts; verification stops. Canceling startup does not poison later key refresh. Another live caller can use the shared key fetch successfully. |
| Bounded shared JWKS fetch | A blocked fetch with no live caller ends under the existing two-second HTTP client timeout; subsequent verification recovers. This deliberately does not cancel a shared fetch on behalf of one caller. |
| API disconnect during JWKS wait | Real PATCH with a body cancels its authentication wait and never acquires a database connection. This regression initially failed and passes after the correction below. |
| Input-error precedence | Unauthenticated, bounded invalid JSON still yields 401; an oversized unauthenticated body yields 413. Existing authenticated media/schema/ownership/revision/cursor tests still pass. |

The new API-level test initially finished only after the shared key-fetch timeout, with an uncanceled handler context. In HTTP/1, the body was unread during authentication, delaying the server's background disconnect read. The Go 1.27.1 server registers that read on body EOF; inspect [pinned server source](https://github.com/golang/go/blob/go1.27.1/src/net/http/server.go) and [request-context semantics](https://pkg.go.dev/net/http#Request.Context). The owner approved buffering up to 64 KiB within the existing operation deadline before authentication, then applying media-type/JSON/schema validation afterward. Oversized/failed reads may now return 413/400/503 before authentication. No JWT caching or skipped authorization was added.

The before-correction failing output is retained locally. It ran the new regression against F09's pre-correction registration handler. Corrected full Linux race output covers all packages against PostgreSQL 18.6, including real transactions, runtime grants, concurrency, migration failure/rollback and pool recovery. Windows output covers platform-independent/network tests; database tests are explicitly skipped there. Windows/Linux vet/build and documentation checks passed. Test grace/deadlines are synchronization bounds, not new product SLOs. The owner selected public code/report publication with raw records/profiles and detailed machine files kept local; see [summarized verification](validation/f10-checks.json).

## Registration observations

Measured local source: `79ac8323c830447b1d3aa3cb389ef7109584a344`, from a clean checkout; retained in the unpushed local evidence branch. The public runtime, tests and harness in [`7ed1732634c94bdf713b595a46a28cdf73da98c2`](https://github.com/octieght18/forge/commit/7ed1732634c94bdf713b595a46a28cdf73da98c2) are byte-identical; publication history was rebuilt to keep raw evidence private. Exact phase timestamps remain in local conditions files. The four requests are workload POST, version POST, workload GET and version GET, in sequence per client. Each journey uses a distinct synthetic workload/version under one signed developer identity.

| Clients | Measured requests / journeys | Request p50 / p95 / p99 (ms) | Journey p50 / p95 / p99 (ms) | Request errors | Completion rate (requests/s) |
|---|---:|---:|---:|---:|---:|
| 1 | 17,948 / 4,487 | 0.934 / 1.393 / 1.717 | 3.279 / 3.973 / 4.421 | 0 / 17,948 | 1,196.3 |
| 2 | 32,232 / 8,058 | 1.025 / 1.614 / 1.976 | 3.643 / 4.500 / 4.977 | 0 / 32,232 | 2,148.5 |

Actual measured windows, including drain, were 15.002785 s and 15.002215 s. Neither level had a failed journey or transport error. One-client statuses were 8,974 each of 200 and 201; two-client statuses were 16,116 each. Maximum request latency was 2.907/7.318 ms; maximum journey latency was 5.822/9.509 ms. Warmup/preflight are retained and excluded from those distributions. No measured outcome or outlier was discarded. The [published summary](validation/f10-registration-summary.json) includes per-operation distributions and explicit error denominators.

Including preflight and warmup, the isolated databases contained 5,949 and 10,746 workload/version pairs before cleanup, matching the successful journeys. Their disposable databases/roles were removed afterward. Existing Forge databases, secrets and product records were not used by this harness.

## CPU, heap and tradeoffs

The test process is uncapped, Linux/amd64 Go 1.27.1 with GOMAXPROCS 16, running in native Ubuntu 26.04 WSL2 on this Ryzen 7 5800H PC. WSL sees 16 CPUs, 7,820,960 KiB RAM and 2 GiB swap. Native PostgreSQL 18.6 runs separately on test port 55435; API HTTP uses an OS-assigned loopback port. The fixture pool ceiling is twelve connections; API/client deadlines are five/ten seconds. Setup and initial key retrieval finish before measurement. Logs are formatted to `io.Discard`; no race instrumentation, containers, model calls or document retrieval run during profiling. Detailed machine and per-level conditions files remain local. No retained native Forge units were active at the post-run snapshot. After profiling, the existing native stack started with the correction and passed API/identity Windows loopback readiness checks, retaining existing data/secrets.

Combined-process CPU sampling captured 11.19 CPU-seconds for one client and 21.15 for two across their fifteen-second windows. System calls and runtime scheduling/futex work dominate flat samples; API-filtered stacks also show RSA verification, allocation and validation. These are sampled stacks, not isolated database wait times or proof of one bottleneck. Repeated JWT verification and schema/ownership checks remain intact.

Exact post-GC combined-process heap was 3,787,504 → 2,016,104 bytes for one client and 3,320,264 → 2,246,352 for two. Total allocation deltas were 748,193,840 and 1,371,207,872 bytes, with 284/523 GC cycles: roughly 41.7/42.5 kB allocated per measured request, including the client and recorder. Allocation profiles show JOSE JSON decoding, HTTP headers, JSON/schema validation and body reads. Sampled retained heaps include runtime allocations, schema objects, pool and HTTP buffers. Short before/after heap observations cannot prove absence of leaks or predict sustained RSS. No performance optimization is justified by this small sample alone; the observed cancellation defect was fixed with a regression test.

Profiles include API, client, signed issuer fixture, recorder and Go runtime in one process. PostgreSQL CPU/memory is excluded. API CPU labels aid inspection; heap profiles are not API-only. Free model throughput, real Keycloak capacity, native service resource limits, network/TLS effects, multi-owner contention and sustained traffic are unmeasured. These results do not establish an SLO or capacity ceiling or supersede F08. Numerical targets still require owner review.

## Local evidence and public verification

Raw JSONL (losslessly compressed), CPU profiles, before/after heap profiles, detailed conditions/machine metadata, pprof top reports, test outputs and the SHA-256 manifest remain under ignored `docs/profiles/f10-2026-10-07/` on the owner's PC. The original observation directory is `/tmp/forge-f10-full-20261007` in Ubuntu WSL. The unpushed branch `local/f10-private-evidence` retains the measured source and original artifact commit. Those commits/files are excluded from public main history. Public readers can reproduce the [procedure](profile-procedure.md) and inspect the [summarized results](validation/f10-registration-summary.json); they cannot independently inspect this PC's raw evidence from GitHub.

Normal tests skip profiling unless explicitly enabled. CI runs a separate 100 ms/300 ms smoke per level, verifies raw summarization and zero observed errors, and retains the existing real Keycloak authorization-code/native restart checks. Hosted validation is recorded on the completed Wekan card against the exact published commit. Raw CI profiling files are temporary runner files, without an artifact-upload step.
