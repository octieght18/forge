# Network and admission policy — F38

[D53](decision-log.md#d53--f38-network-and-policy-as-code) records the boundary. The architecture preferred Kyverno for Kubernetes resource policy. This repository uses that choice.

## Admission

[kyverno-privileged.yaml](../deploy/policy/kyverno-privileged.yaml) is a `ClusterPolicy` with `validationFailureAction: Enforce`. It rejects a privileged container, privilege escalation, host namespaces, and a host path volume.

Kyverno is not installed in this environment. The check that runs is `internal/policy` during `go test`, plus [deploy/policy/test_policy.py](../deploy/policy/test_policy.py) against the kind resource generator. A privileged or host-network pod fails those checks. That is CI rejection, not a live admission decision from a cluster.

## Network

[network-default-deny.yaml](../deploy/policy/network-default-deny.yaml) selects every pod in `forge-local` and sets policy types Ingress and Egress with no allow rules. That is the default-deny document.

Further documents allow:

| Path | Rule |
|---|---|
| Gateway and in-cluster API, PostgreSQL, and Keycloak | Ingress and egress between pods in `forge-local`. The kind proxy is a sidecar on the API pod, so it is not a separate peer |
| DNS | UDP and TCP 53 to `kube-dns` |
| Model | TCP 443. The worker's configured route is OpenRouter. NetworkPolicy cannot name that host, so the port is the exception |
| Tool | No rule. `corpus.search` and `corpus.read` are a stdio process, not a network client |

Kind's current network plugin does not enforce NetworkPolicy. These manifests were not applied. No connection was attempted, so no denied path was observed. Default-deny is not active on this kind cluster.
