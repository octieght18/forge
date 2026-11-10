# Capacity and cost model — F49

[D64](decision-log.md#d64--f49-capacity-and-cost-model) records the boundary. This page combines **published** Forge measurements with a **public-list price model**. It does not claim a production SLO or a live cluster capacity ceiling. `terraform apply` was not run.

## Hardware and traffic assumptions

| Context | Hardware | Traffic / load |
|---|---|---|
| F08 native baseline (7 Oct 2026) | AMD Ryzen 7 5800H; WSL2 Ubuntu; 16 logical CPUs; ~7.8 GiB WSL RAM | 5 retained startups; 10 sequential registration journeys; 1 occupied-port recovery |
| F10 registration profile (7 Oct 2026) | Same PC; uncapped Linux Go test; GOMAXPROCS 16; disposable PostgreSQL | 1 and 2 clients; 5 s warmup + 15 s measure each |
| F19 / F29 / F45 / F46 / F47 | Owner PC or CI disposable DBs as documented per ticket | In-process / SDK / disposable-DB samples |
| F49 session re-check (10 Nov 2026) | Cloud agent VM: Intel Xeon, 4 vCPU, ~15.6 GiB RAM, Linux 6.12, Go 1.27.1 | Re-ran existing Go suites only; no new production profile |

Product process caps (not distributed): **200 req/s**, **100 workloads**, **100 pending runs** per API process; limited requests return `429` with `Retry-After: 1`.

## Concurrency, latency, saturation (measured / reused)

| Signal | Value | Source |
|---|---|---|
| Registration request completion | 1 client **1196.3 req/s**; 2 clients **2148.5 req/s** | [F10 summary](validation/f10-registration-summary.json) |
| Request p50 / p95 (1 client) | 0.934 / 1.393 ms | F10 |
| Request p50 / p95 (2 clients) | 1.025 / 1.614 ms | F10 |
| Journey p50 (1 / 2 clients) | 3.279 / 3.643 ms | F10 |
| Combined-process CPU in 15 s window | 11.19 / 21.15 CPU-seconds (1 / 2 clients) | [F10 validation](f10-validation.md) |
| Allocated bytes per measured request (approx.) | ~41.7 / 42.5 kB including client+recorder | F10 |
| Post-GC heap (combined process) | ~2.0–2.2 MiB after GC in those samples | F10 |
| Native startup median | 66.468 s (max 71.100 s) | [F08](f08-baseline.md) |
| Native registration journey median | 0.022 s | F08 |
| Template provision accept (3 kinds) | register/deploy/provision medians 3.853 / 3.540 / 4.041 ms | [F19](f19-validation.md) |
| Failure recovery (controller fake client) | 9.4041 ms; 1 fail + 2 success | F19 / F18 |
| Durable recovery samples | dependency 43361 µs; uncertain 12993 µs; cancel 1816 µs; replay 535 µs | [F29](f29-validation.md) |
| Failure injection durations | worker 38118; tool/model 30695; rollout 33; DB 17329; quota 16572 µs | [F47](validation/f47-failure-injection.json) |
| Balanced local targets | all recorded checks **passed** on 10 Nov 2026 re-assessment | `scripts/check-performance-targets.py` |

Saturation reading: two clients roughly doubled request throughput while p95 stayed under the 5 ms Balanced local limit. That harness excludes PostgreSQL process CPU, real Keycloak, TLS, model calls, and multi-owner contention. The product 200 req/s cap is far below the uncapped F10 fixture rate; treat 200 req/s as the enforced demo ceiling, not the F10 peak.

## Database growth (from published F10 counts)

Including preflight and warmup, disposable databases held **5,949** and **10,746** workload/version pairs for the one- and two-client profiles before cleanup. Exact on-disk PostgreSQL bytes were not published. Growth therefore scales with journey count (one workload + one immutable version per journey), not with a measured GiB/day figure.

Retention ([retention.md](retention.md)): executions stay in PostgreSQL and Temporal; evidence and audit stay in-process and are lost on process exit. Backup dump scope excludes Temporal history ([backup.md](backup.md)).

## Cost model — price date and sources

**Price date:** 10 October 2026 snapshot for AWS us-east-1 on-demand list assumptions used below (cross-checked against public AWS EKS pricing pages on 10 November 2026). **Model token price date:** 5 October 2026 OpenRouter catalog listing for `google/gemma-4-26b-a4b-it:free` (zero prompt/completion token prices; [architecture.md](architecture.md)).

Sources: [Amazon EKS pricing](https://aws.amazon.com/eks/pricing/), EC2/VPC public list assumptions consistent with the module shape in [terraform-eks.md](terraform-eks.md) (1× `t3.small` scalable to 2, 1 NAT gateway). **No AWS account was billed.** Prices change; re-check before budgeting.

| Line item | Unit price (assumption) | Monthly at 730 h | Notes |
|---|---:|---:|---|
| EKS control plane (standard support) | $0.10 / cluster-hour | $73.00 | Not provisioned |
| EC2 `t3.small` ×1 | $0.0208 / hour | $15.18 | Module default node; second node optional |
| NAT gateway hourly | $0.045 / hour | $32.85 | Data processing extra (~$0.045/GB) |
| EBS gp3 20 GiB (assumption) | $0.08 / GiB-month | $1.60 | Not measured from a live volume |
| Public IPv4 / LB / transfer | omitted | — | Not estimated here |
| OpenRouter free Gemma tokens | $0.00 / 1K tokens | $0.00 | Free route only; no paid fallback |
| Local kind / native stack | $0 cloud | operator electricity/time | Current demonstrated path |

**Illustrative idle cloud floor (1 node, no transfer):** about **$73 + $15.18 + $32.85 + $1.60 ≈ $122.63 / month**. A second `t3.small` adds ~$15.18. This is not an invoice.

### Per-request and per-workload unit cost (model)

Utilization scenario for attribution (stated, not measured production):

- Assume the illustrative cluster runs continuously ($122.63 / month).
- Assume **10%** of wall time is attributable product API work (sensitivity below).
- Attribute that share across requests at the **enforced** 200 req/s cap only during busy seconds — for a conservative monthly request estimate use **1 million registration-class requests / month** (planning figure, not traffic we observed in cloud).

| Unit | Formula | Result |
|---|---|---|
| Infra $ / request @ 1e6 req/mo, 100% attribution | 122.63 / 1e6 | **$0.000123** |
| Infra $ / request @ 1e6 req/mo, 10% attribution | (122.63 × 0.10) / 1e6 | **$0.0000123** |
| Model $ / research run (free route) | 0 | **$0** |
| Telemetry $ | in-process meters; no collector billed | **$0** in this model |
| Storage $ / workload row | share of $1.60 EBS assumption; row bytes unmeasured | treat as **≪ $0.01** until sized |
| F19 self-service provision accept (3 workloads) | infra attribution negligible at ms scale; cloud not applied | local disposable DB only |

**Sensitivity:** doubling nodes ≈ +$15/mo per node; NAT data at 100 GB/mo ≈ +$4.50; moving EKS to extended support ($0.60/h) replaces $73 with $438 and dominates the bill; losing the free model route is refused by product policy rather than priced in.

## What was not measured

- Live EKS node CPU/memory saturation
- Distributed rate-limit throughput
- OpenRouter free-tier RPM against this account
- Production p99 under TLS and a real gateway
- Continuous archive / PITR cost

Machine-readable companion: [validation/f49-capacity-cost.json](validation/f49-capacity-cost.json).
