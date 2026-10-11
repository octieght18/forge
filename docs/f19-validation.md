# F19 verification — 11 October 2026

F19 renders a research service, a tool-calling agent, and a resource-constrained job, then registers, deploys, and requests provisioning for each one. [D33](decision-log.md#d33--f19-three-template-onboarding-measurement) records the boundary. All three stay on `research.v1`. The job's `resources.json` records profile `small-v1` and does not apply it.

Checked with Go 1.27.1. Windows ran the template and CLI packages. A disposable PostgreSQL 18 on Ubuntu then ran `TestThreeTemplatesRequestProvision` with `FORGE_REQUIRE_DB_TESTS=1`. The retained API on port 8081 was down, so these timings use the registration handler and a temporary database. They are separate from the [F08 native loopback samples](f08-baseline.md).

```text
go test -count=1 ./internal/template ./internal/cli
go test -count=1 -run TestThreeTemplatesRequestProvision ./internal/httpapi
go test -count=1 -run TestFailureRecoveryConvergesWithoutDuplicates ./internal/environment
```

## Three onboarded workloads

| Kind | Register (ms) | Deploy (ms) | Provision request (ms) | Workload |
|---|---:|---:|---:|---|
| service | 11.546 | 5.390 | 5.304 | `80965849-6458-40b2-9f16-44fe00fe177f` |
| agent | 3.853 | 3.519 | 4.041 | `3e540b42-75b7-47ab-a8f8-19a0d3a66cd1` |
| job | 3.312 | 3.540 | 3.258 | `cbe72fac-017a-4739-906f-8738646c10b6` |

Each provision returned `202` with status `accepted`. The supported count in this run is 3 kinds and 3 workloads. The API publishes no higher capacity number. F08's ten registration fixtures did not request provisioning.

Sorted medians for these three samples are 3.853 ms to register, 3.540 ms to deploy, and 4.041 ms to accept provisioning. Ranges are 3.312–11.546 ms, 3.519–5.390 ms, and 3.258–5.304 ms. F08's native medians were 6.790 ms to create a workload and 7.912 ms to create a version, over ten sequential loopback requests. The paths differ, so this table does not claim a faster platform.

## Manual steps

After a token exists, F08's measured journey was create workload, create version, read workload, and read version. Environment state stayed `not_requested`.

A developer now has these local and API steps:

1. `forge template render` for `service`, `agent`, or `job`.
2. Register the generated workload.
3. Deploy the generated version.
4. `forge provision`, which returns a status URL.

These developer tasks are covered by the template instead of a hand-written document:

- The research version body.
- The `ForgeEnvironment` field names, with `<workloadID>`, `<issuer>`, and `<subject>` still to fill.
- Namespace, quota, limit range, and worker account creation. The controller performs that work after an operator applies the environment.

Login, the private token file, and `environment.ps1 apply` remain manual. Developers still receive no kubeconfig. Run submission and Kubernetes Job execution remain unavailable. The human worksheet is still empty. The ten paired manual-versus-self-service trials from the baseline procedure were not run.

## Failure recovery

`TestFailureRecoveryConvergesWithoutDuplicates` passed in this check. Recovery was 9.4041 ms, with 1 reconciliation failure and 2 successes. The published F18 sample of the same test was 7.1103 ms. Both are in-process fake-client observations under the 5 second hang guard. An operation that already observed `Failed` stays failed. The installed controller still does not write operation rows.

Documentation checks reported 49 Markdown documents and 359 relative links. The PowerShell checker parsed 7 scripts and 29 documented PowerShell blocks.
