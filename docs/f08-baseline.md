# F08 native developer-experience baseline

Measured on **7 October 2026** using the owner-approved D21 [procedure](baseline-procedure.md). The complete corrected protocol has five retained-data starts, ten authenticated registration/read journeys (40 HTTP requests) and one occupied-port detection/recovery case. Every required operation passed or produced its expected failure. These are observations on this PC; performance thresholds remain pending owner review.

| Measurement | Completed samples | Median (seconds) | Min–max (seconds) |
|---|---|---:|---:|
| Retained-data startup | 5/5 successful | 66.468 | 64.966–71.100 |
| Authenticated registration/read journey | 10/10 successful | 0.022 | 0.020–0.092 |
| Automated real PKCE setup (separate) | 1/1 successful | 2.166 | 2.166–2.166 |
| Occupied API port detection | 1 expected address-in-use failure | 0.373 | 0.373–0.373 |
| Startup/readiness after fixture release | 1/1 successful | 64.744 | 64.744–64.744 |

Each registration journey creates one synthetic workload and immutable version, then reads both and checks matching IDs. All ten distinct fixtures are retained. Request-only timings include response body receipt, excluding subsequent parsing/checks; the complete journey includes those checks. Requests are sequential with a shared client session. No model or workflow runs occur.

| Request step | Successful samples | Median (milliseconds) | Min–max (milliseconds) |
|---|---|---:|---:|
| create_workload | 10/10 | 6.790 | 6.173–51.901 |
| create_version | 10/10 | 7.912 | 7.418–16.316 |
| read_workload | 10/10 | 1.573 | 1.364–1.77 |
| read_version | 10/10 | 1.540 | 1.279–2.238 |

## Raw evidence and reproducibility

The canonical [64 raw records](baselines/f08-2026-10-07/raw.jsonl), [metadata](baselines/f08-2026-10-07/metadata.json), [summary](baselines/f08-2026-10-07/summary.json) and [supplemental software/limits snapshot](baselines/f08-2026-10-07/runtime-conditions.json) retain timestamps, monotonic durations, outcomes, commands and fixture IDs. No outlier is removed. Stop durations are recorded separately. The exact measured source is [eb88e27308fbb9df0b8aa0f2a91f14317e56a6e9](https://github.com/octieght18/forge/commit/eb88e27308fbb9df0b8aa0f2a91f14317e56a6e9), with a clean source tree at capture and harness SHA-256 in metadata. The later publication commit adds artifacts/documentation only.

The [harness](../scripts/measure-baseline.ps1) and [summary tool](../scripts/baseline/summarize.py) preserve outcomes and return failure for incomplete/unexpected required results. Five statistics tests cover medians/outliers, visible failed trials, empty/incomplete samples, separate expected failures and invalid/duplicate raw records. PowerShell parsing passed locally. CI additionally checks both tools alongside the existing Windows/Ubuntu, PostgreSQL/race/restart and native real-login lifecycle jobs; exact successful published CI evidence is recorded on the F08 card after those jobs finish.

## Preserved first attempt

The first attempt at [1f73d9a729a3e5ebdcbbf607eba889107be91044](https://github.com/octieght18/forge/commit/1f73d9a729a3e5ebdcbbf607eba889107be91044) completed five starts, then its first registration POST correctly received HTTP 422: the harness generated an uppercase timestamp separator in a name requiring a lowercase slug. This was an invalid benchmark fixture, not a server regression. It created no workload. The fix lowercases the generated timestamp and validates all planned names against the actual OpenAPI name constraints before starting trials.

All [13 raw records](baselines/f08-2026-10-07/attempt-01/raw.jsonl), [original metadata](baselines/f08-2026-10-07/attempt-01/metadata.json) and [incomplete summary](baselines/f08-2026-10-07/attempt-01/summary.json) are retained separately. Its five successful startup observations have median **67.383s**, range **65.939–78.782s**. Its registration and HTTP groups explicitly contain one failure each; they describe the same failed request at different levels. Missing samples stay incomplete with null statistics. They are not pooled into the corrected protocol or hidden as zero timings. The complete protocol was repeated into a new directory after correction.

## Conditions and boundaries

AMD Ryzen 7 5800H, eight physical cores/16 logical processors; Windows 11 Home build 26300 with 15.405 GiB visible memory. Ubuntu 26.04 LTS under WSL2, kernel 6.18.33.2-microsoft-standard-WSL2, 7,820,960 KiB memory and 2 GiB swap, 16 visible logical CPUs. PowerShell 7.6.5; Go 1.27.1 linux/amd64; Python 3.14.4; PostgreSQL 18.6; Keycloak 26.8.0; OpenJDK 21.0.12.1; systemd 259.5. API, Keycloak and PostgreSQL are native loopback services with one CPU equivalent each and memory caps of 256 MiB, 1,536 MiB and 1 GiB respectively. Host Codex/Wekan and other uncontrolled activity remain a source of variation; no stage-by-stage latency attribution is claimed.

The canonical attempt begins with existing services active and stops them before every startup. Data, package prerequisites, archive and build caches are retained; caches are not reset between attempts. Startup includes WSL entry, Go builds, explicit migrations/grants, dependency/API readiness and Windows connectivity checks. It excludes package installation/download, fresh cluster/realm initialization and human work. The separate PKCE number automates the real login form/code exchange; it is not human login or visual usability timing.

Port 8081 is occupied only by the harness's isolated socket fixture. Native startup fails with the specific address-in-use diagnostic; unrelated startup failures cannot satisfy this scenario. The harness releases its own fixture and restarts successfully without deleting data. Recovery timing begins **after** automatic fixture release, excluding diagnosis/decision time. The native stack remains ready at completion. This is orderly programmatic recovery, not a crash/backup recovery objective.

## Pending human and provisioning evidence

The [human worksheet](baseline-human-worksheet.json) deliberately contains null timings. No person has completed measured onboarding/typing, and no developer-effort saving is claimed. The original ten-manual/ten-self-service provisioning comparison remains deferred until provisioning exists; the procedure records how to match prerequisites/readiness and retain steps, failures and recovery. Registration is not provisioning.

Five startup and ten sequential journeys do not establish concurrent capacity, throughput, percentiles, production reliability or an SLO. The owner must review these observations before choosing numerical speed/capacity/recovery thresholds. The accepted first-release API/durable-workflow scope is unchanged.
