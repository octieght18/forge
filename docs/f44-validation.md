# F44 verification — 13 October 2026

F44 implements canary promotion from the local overlay to staging. [D59](decision-log.md#d59--f44-canary-promotion) records the boundary. The procedure is in [promotion.md](promotion.md).

`TestPromoteCopiesLocalPinToStaging` copied `local-overlay` into a temporary staging pin. `TestPromoteRefusesCloudAndLeavesStagingUntouched` refused `cloud`, `eks`, and `gke` and left staging absent. Promotion to the paid cloud environment did not run. The command does not open PostgreSQL or Temporal.

`CanaryAccepts(1000, 10)` is within the 1 percent ceiling and `CanaryAccepts(1000, 11)` is not. No live staging window was collected, so that ceiling was not measured against a cluster.

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/delivery ./cmd/forge-promote
python3 scripts/check-docs.py
```
