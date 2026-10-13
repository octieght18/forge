# F38 verification — 13 October 2026

F38 selects Kyverno and adds a CI rejection for privileged manifests. [D53](decision-log.md#d53--f38-network-and-policy-as-code) records the boundary. The manifests and the kind limit are in [policy.md](policy.md).

Kyverno was not installed. Docker, kind, and kubectl were not available, so nothing was applied to a cluster. The CI checker rejected a fixture pod with `hostNetwork`, `privileged`, `allowPrivilegeEscalation`, an added `SYS_ADMIN` capability, `hostPath`, and `hostPort`. The same checker accepted a non-root pod that drops all capabilities. The kind resource generator produced no findings.

The network documents contain a default-deny policy, same-namespace ingress and egress, DNS on port 53, and TCP 443 for the model route. There is no tool egress rule. `NetworkIntent` records `Enforced=false` and `DeniedObserved=false`. No packet was sent, and no denied network path was observed.

Checked with Go 1.27.1:

```text
go test -count=1 ./internal/policy
python3 -m unittest discover -s deploy/policy -p 'test_*.py'
python3 scripts/check-docs.py
```
