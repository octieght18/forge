# Performance and developer-impact benchmark — F54

[D69](decision-log.md#d69--f54-performance-and-developer-impact-benchmark) records the boundary. This benchmark **reuses published samples**. It does not fabricate a new manual baseline. The product-brief “10 manual vs 10 self-service” provisioning comparison remains **unclaimed**.

## Methodology

1. Take F08 native registration/startup observations as the pre-provisioning developer path.
2. Take F19 three-template register/deploy/provision-accept timings as the self-service provisioning request path (API handler + disposable DB; not native loopback).
3. Take F18/F19 controller recovery and F29/F47 execution recovery as reliability/recovery evidence.
4. Publish variance as min–max / dual samples already recorded. No outlier deletion.
5. State environment differences explicitly; do not pool incompatible harnesses into one “speedup” headline.

## Manual-setup baseline (published F08)

| Metric | Samples | Median | Min–max | Source |
|---|---:|---:|---:|---|
| Retained-data startup | 5/5 | 66.468 s | 64.966–71.100 s | [f08-baseline.md](f08-baseline.md) |
| Authenticated registration journey | 10/10 | 0.022 s | 0.020–0.092 s | F08 |
| create_workload / create_version | 10/10 | 6.790 / 7.912 ms | see F08 | F08 |
| Occupied-port detection / recovery | 1 / 1 | 0.373 s / 64.744 s | single samples | F08 |
| Human onboarding worksheet | — | null | — | still unmeasured |

After F08, environment state stayed `not_requested`. Manual namespace/YAML authoring was not timed.

## Self-service provisioning request (published F19)

| Kind | Register ms | Deploy ms | Provision accept ms |
|---|---:|---:|---:|
| service | 11.546 | 5.390 | 5.304 |
| agent | 3.853 | 3.519 | 4.041 |
| job | 3.312 | 3.540 | 3.258 |

Medians across three samples: register **3.853 ms**, deploy **3.540 ms**, provision accept **4.041 ms**. Supported workload kinds in that run: **3**. Paths differ from F08 native loopback, so this table does **not** claim the platform became faster than F08.

### Steps removed vs remaining (developer impact)

Removed from hand-authored docs (template/CLI): research version body skeleton; `ForgeEnvironment` field names (placeholders remain); namespace/quota/limit/worker account creation **after** operator apply.

Still manual: login + private token file; `environment.ps1 apply`; no kubeconfig for developers; human worksheet empty; paired 10/10 manual-vs-self-service trials **not run**.

## Workload counts, reconciliation, execution reliability

| Signal | Result | Source |
|---|---|---|
| Distinct template kinds onboarded | 3 | F19 |
| Provision HTTP status | `202` `accepted` ×3 | F19 |
| Controller recovery | 9.4041 ms; 1 fail + 2 success; no duplicate namespace | F19/F18 |
| Durable dependency failure | 43361 µs; Retrieve×2; compensate×1; 1 evidence | F29 |
| Replayer after history | 535 µs; 0 activity entries | F29 |
| F47 five scenarios | all recovered; isolated; duplicate side effects 0 | [f47-failure-injection.json](validation/f47-failure-injection.json) |

## Registration concurrency (F10) — context only

1 client: 1196.3 req/s, p95 1.393 ms, 0 errors. 2 clients: 2148.5 req/s, p95 1.614 ms, 0 errors. Product cap 200 req/s. Not a production SLO.

## Limitations

- No new baseline was collected on 10 November 2026 beyond re-running Balanced target assessment (passed) and Go recovery suites during F49/F53.
- No live worker kill; no Temporal server; no cloud cluster capacity.
- Do not advertise a single “X× faster onboarding” number.

Machine-readable pointers: [validation/f54-benchmark.json](validation/f54-benchmark.json).
