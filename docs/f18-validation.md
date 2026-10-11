# F18 verification — 11 October 2026

F18 checks that the environment controller converges after an API outage, a controller restart, and a partial boundary. [D32](decision-log.md#d32--f18-controller-failure-recovery) records the boundary. The registration HTTP server stays up. The controller's identity lookup is the product API it can fail to reach. A replacement reconciler uses the same client. Quota and limit range are deleted after the namespace exists.

Checked with Go 1.27.1 on Windows:

```text
go test -count=1 ./internal/environment ./internal/telemetry ./internal/provision
```

`TestFailureRecoveryConvergesWithoutDuplicates` records the sequence:

- While identity lookup returns an error, no namespace is created and the pass is `IdentityUnavailable`.
- The operation that observed that `Failed` phase stays `failed` with `controller_failed`. A later `Ready` observation does not reopen it.
- After lookup recovers, the quota and limit range are removed and a new reconciler continues.
- The final state is one namespace, one `boundary` quota, one `containers` limit range, and one `worker` account. Each is owned by the environment. The worker account does not automount a token.
- The next pass keeps the environment resource version and the namespace UID.
- A new provision operation reaches `ready` with matching generations.

The measured recovery, from lookup recovery until `BoundaryReady`, was 7.1103ms. The in-process view recorded 1 reconciliation failure and 2 successes. The test lookup waits 1ms so the Ready sample is visible on a clock that reports a faster pass as zero. Production lookup does not add that wait. The test requires the recovery to stay under 5 seconds. That bound is a hang guard for the fake client, not a capacity target.

The test identity lookup is the only delay. No kind node, PostgreSQL, or HTTP server was stopped for this check. [F13](f13-validation.md) still covers whole-node restart, admission, and real finalization. A failed operation is not rewritten when the boundary recovers, and the installed controller still does not write operation rows.

Documentation checks reported 48 Markdown documents and 351 relative links. The PowerShell checker parsed 7 scripts and 29 documented PowerShell blocks.
