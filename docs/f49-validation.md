# F49 verification — 10 November 2026

F49 records capacity observations and a dated cost model. [D64](decision-log.md#d64--f49-capacity-and-cost-model) records the boundary. Narrative: [capacity-cost-model.md](capacity-cost-model.md). Raw companion: [validation/f49-capacity-cost.json](validation/f49-capacity-cost.json).

## What ran in this session

```text
python3 scripts/check-performance-targets.py --baseline-summary docs/baselines/f08-2026-10-07/summary.json --profile-summary docs/validation/f10-registration-summary.json
go test -count=1 -timeout 180s -run TestFailureInjection ./internal/failure
go test -count=1 -run 'TestRollbackRestoresPinAndKeepsExecutions|TestDurableRecoveryMeasurements' ./internal/delivery ./internal/execution
python3 scripts/check-docs.py
```

Balanced local target reassessment **passed**. Failure-injection (non-DB scenarios), rollback, and durable-recovery tests **passed** on this cloud VM (4 vCPU, ~15.6 GiB RAM). No new F10 multi-client profile was collected here.

## Capacity summary

Reused F10 peaks: **1196.3** and **2148.5** req/s at 1 and 2 clients with p95 under 5 ms. Product cap remains **200 req/s** per process. F10 disposable DB held 5,949 / 10,746 workload/version pairs after the profiles. F08/F19/F29/F47 timings are cited in the model doc without inventing live cluster saturation.

## Cost summary

Illustrative us-east-1 idle floor for the unapplied EKS module shape ≈ **$122.63 / month** (price assumptions dated **2026-10-10**, EKS standard support confirmed on public pricing pages **2026-11-10**). Free Gemma token price **$0**. Infra attribution examples at 1e6 req/mo are in the JSON. **No cloud resources were created and no spend occurred.**

## Limits

No production SLO. No terraform apply. No OpenRouter call. PowerShell documentation checker was not run on this Linux VM.
