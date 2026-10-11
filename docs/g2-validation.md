# G2 gate — measured self-service Kubernetes provisioning

Recorded on 11 October 2026. [D36](decision-log.md#d36--g2-measured-self-service-provisioning) is the decision. F19 and F20 blocked this gate and are in Done. G2 is a review of those results plus the developer request path.

## Idempotent reconciliation and recovery

`TestReplayAndDriftRepair` keeps the environment resource version across repeated ready passes and repairs quota and token drift. `TestSubmitIsIdempotentAndRemoveIsRepeatable` submits one environment, submits it again without changing the object, refuses a different owner, and removes it twice.

`TestFailureRecoveryConvergesWithoutDuplicates` measured recovery at 7.1103 ms when it was published with F18, and 9.4041 ms when F19 ran it again. Both samples recorded 1 reconciliation failure and 2 successes. The final boundary was one namespace, one quota, one limit range, and one worker account.

## Published workload count and setup

F19 onboarded 3 workloads: service, agent, and job. Register medians and ranges are in [F19 verification](f19-validation.md). The provision-request median was 4.041 ms, range 3.258–5.304 ms. The template covers the research version, the environment field names, and the boundary resources. Login and the private token file remain the developer prerequisites.

## Developer request

`forge provision` accepts the operation and returns `202`. The controller leader, every 15 seconds, reads open operations and submits the `ForgeEnvironment`. Delete and cancel remove that object. The controller then reconciles the namespace. The developer does not run `kubectl` or `environment.ps1`. Operators can still use that command.

The API deployment keeps `automountServiceAccountToken` false. The environment role can read the operation columns and cannot update them, so the status URL can remain `accepted` after the namespace is `Ready`. Operator status reads the environment object. A cluster picks this up when its controller image, `environment-grants.sql`, and controller role match this commit.

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/environment ./cmd/forge-environment-controller
python -m unittest test_environments.py test_resources.py
```

The environment package passed. The controller command has no tests of its own and compiled with the package. The Kubernetes trust tests passed, including the rule that the API pod does not mount a service-account token.

Documentation checks reported 54 Markdown documents and 392 relative links. The PowerShell checker parsed 7 scripts and 29 documented PowerShell blocks.
