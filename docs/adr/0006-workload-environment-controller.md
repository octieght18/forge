# ADR 0006 — Separate operator environment controller

**Status:** Accepted by owner replies on 8 October 2026; implemented in F12a/F13.

## Context

F11/F12 were adapted to research/run contracts and command reconciliation while Kubernetes provisioning was deferred. F07a later deployed API/PostgreSQL/Keycloak in kind. The owner now restored the CRD/controller prerequisite and selected one namespace per workload, with resource/runtime boundaries before network isolation.

## Decision

Use a separate Go controller-runtime 0.25.2 controller aligned with Kubernetes client 0.37 and the deployed Kubernetes 1.37 cluster. Keep F12 command reconciliation unchanged. Use cluster-scoped ForgeEnvironment v1alpha1 with immutable registered identity and fixed small profile, API-server schema/transition validation and separate observed status. Cluster scope allows valid namespace ownership references. [Controller-runtime version alignment](https://github.com/kubernetes-sigs/controller-runtime) and [Kubernetes ownership rules](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/) support these choices.

Only Kubernetes operators submit/delete environment intent. A separate PostgreSQL role verifies registered workload/owner identity through column-limited SELECT, without API credentials or product-write grants. Deterministic names, ownership UID checks, conflict-aware updates, persisted finalizers and bounded reconciliation support interruption recovery. Conditions describe boundary state separately from research execution.

Provision namespace, quota, LimitRange and a worker account. Restricted admission and no implicit token mounts are defaults; no worker image, run, public environment endpoint or network-policy enforcement is supplied. Cleanup refuses foreign ownership and persistent storage and waits for Kubernetes/foreign finalizers. [Environment guide](../execution-environments.md) specifies the profile and operational contract.

## Alternatives, cost and reversibility

An operator-run provisioner would reduce controller dependencies/privilege but would require explicit repeated invocation; the owner chose a continuously reconciling controller. Direct client-go was offered as an alternative to controller-runtime. A namespaced CRD cannot validly own the cluster-scoped namespace. An immediate environment HTTP API was offered but the owner retained operator-only intent; it would also require an access and durable handoff design.

The Kubernetes dependencies increase module graph, compile time and controller image size. Dynamic namespace permissions make the controller privileged infrastructure; name-prefix code guards are not admission enforcement against a compromised controller. Empty boundaries incur Kubernetes object overhead; no paid services or model requests are introduced.

Stopping the controller preserves intent/resources. Replacing its implementation can preserve CRD version and ownership contract. Changing CRD shape/scope, profile or network plugin requires a separate migration decision. Delete environments through normal finalization before uninstalling the controller/CRD; ordinary stack stop retains them. Native original databases stay available, with no automatic synchronization from kind.

See [D27](../decision-log.md#d27--restored-environment-controller-and-workload-boundaries) and [validation](../f13-validation.md).
