# Balanced local performance targets

The owner accepted **Balanced** on 7 October 2026 after reviewing F08/F10 observations. D23 records the decision. These are local acceptance limits for repeating the existing protocols under comparable conditions on this PC. They do not define production availability, a capacity ceiling, cloud performance, fresh-installation speed or crash/backup recovery objectives.

| Measure | Accepted maximum | Recorded observation |
|---|---:|---:|
| Each retained-data native startup | 90 seconds | 71.100 seconds in the canonical run; 78.782 including the earlier attempt |
| Each native four-request registration/read journey | 150 ms | 92.332 ms |
| Request p95 at each one/two-client profiling level | 5 ms | 1.393 / 1.614 ms |
| Occupied API port detection | 1 second | 0.373 seconds |
| Readiness after releasing the occupied-port fixture | 90 seconds | 64.744 seconds |

Require zero unexpected errors in healthy operations, including profile preflight/warmup/measurement. Expected occupied-port failure and intentional authentication/fault tests retain their expected outcomes. Existing persistence, ownership, rollback, cancellation and drain checks remain mandatory. Limits are inclusive: equality passes; any measured exceedance fails the relevant check. Native maxima apply to every trial, not its median. Profiling p95 applies separately to each concurrency level, not to pooled requests.

Native conditions: five retained-data starts, ten sequential authenticated four-request journeys and the occupied-port failure/recovery case through the Windows loopback client, with cached prerequisites, existing data, real Keycloak and the accepted native resource caps. Startup includes builds/migrations/readiness; installation/download and fresh initialization are excluded. Recovery begins after the fixture releases the port; human diagnosis/release time is excluded. Keep the earlier incomplete benchmark attempt separate.

Profiling conditions: exactly one and two clients, each with five seconds of warmup and fifteen seconds of measurement, draining already-started journeys; real registration HTTP and disposable PostgreSQL with signed OIDC fixtures in the uncapped Linux Go test process. This limit applies to that harness; it is not a 5 ms target for the resource-capped native Windows-to-WSL journey. CPU/heap profiles still include the client/fixture/recorder/runtime and exclude PostgreSQL process resource use.

## Assess the existing recorded evidence

[Machine-readable limits](performance-targets.json) and the [checker](../scripts/check-performance-targets.py) implement these rules. This command reads summaries and reports pass/fail without restarting services, generating load or modifying measurements:

```sh
python3 scripts/check-performance-targets.py --baseline-summary docs/baselines/f08-2026-10-07/summary.json --profile-summary docs/validation/f10-registration-summary.json
```

The [recorded assessment](validation/balanced-target-assessment.json) passes all five timing targets and healthy-operation error checks. This is a retrospective assessment of existing observations after target approval, not a new measurement or guarantee about later runs. Original raw records, conditions, latency figures and historical F08 `slo_assessment: not_defined` are preserved. F10's public summary adds all-phase failure counts derived from the same locally retained raw records; its existing measured numbers are unchanged.

## Assess a new comparable run

Repeat the [native baseline procedure](baseline-procedure.md) into a new directory and generate its summary. Repeat the [profiling procedure](profile-procedure.md), then generate a new profile summary with the updated summarizer. From the repository root:

```sh
python3 scripts/check-performance-targets.py --baseline-summary /absolute/path/to/new-native-run/summary.json --profile-summary /absolute/path/to/new-profile-run/summary.json
```

Either summary flag can be supplied independently when checking only that protocol. Exit 0 means all supplied checks pass; exit 1 means a limit/error-count check failed; exit 2 means missing, malformed, incomplete or inapplicable evidence. Short/smoke profiling, missing client levels and mismatched native trial counts cannot pass. Review hardware/software, cached prerequisites and runtime conditions before treating a later run as comparable; the checker does not independently verify that environment. Preserve failed runs and all outliers. Changing limits requires owner consultation.

CI tests the evaluator at exact boundaries, exceedances, failures (including warmup), missing/incomplete samples and smoke exclusion, then assesses the committed historical summaries. It does not apply this PC's timing limits to GitHub runner traffic. The separate F10 CI smoke remains a pipeline/correctness check without performance acceptance. Raw profiles, raw traffic and detailed machine files stay local per D22; public output contains code and aggregate reports only.
