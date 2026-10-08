# Kubernetes execution boundaries — F12a and F13

The owner restored the deferred environment CRD/controller prerequisite and approved one namespace per registered workload on 8 October 2026. [D27](decision-log.md#d27--restored-environment-controller-and-workload-boundaries) records the decisions. This provisions resource/runtime boundaries for later workers. It does not deploy an agent, start a research run, enforce network isolation or add environment fields to the research API.

## Operator journey

Start the [kind stack](kubernetes-local.md) and log in. The helper must remain running through the browser callback. Use a workload ID from the authenticated registration API:

```powershell
./scripts/kubernetes-stack.ps1 up
./scripts/kubernetes-stack.ps1 login
```

Then, in another terminal:

```powershell
$workloadId = '<registered-workload-uuid>'
./scripts/environment.ps1 apply -WorkloadId $workloadId
./scripts/environment.ps1 status -WorkloadId $workloadId
```

`apply` reads the workload using the private login token, copies its verified owner into immutable intent and submits it to Kubernetes. Repeating it keeps the same CRD resource and namespace. Use `-TokenFile` with an absolute **WSL** token path to select another private token. The default is `~/.local/share/forge-kubernetes/tokens/access.json` inside Ubuntu. Tokens expire after five minutes; log in again when the API returns 401.

This is a Kubernetes operator command using the installation's private administrator kubeconfig. OIDC authentication authorizes the product read; it does not grant Kubernetes access. Developers receive no kubeconfig or namespace RBAC grants from Forge. Operators may use their product operator identity to inspect another owner's workload. No public environment mutation endpoint exists.

Delete an execution boundary only when its disposable contents are no longer needed:

```powershell
./scripts/environment.ps1 delete -WorkloadId $workloadId
./scripts/environment.ps1 status -WorkloadId $workloadId
```

Deletion is asynchronous. It removes the managed namespace and its disposable namespaced contents; the product workload, versions and runs remain. A terminating environment exposes the blocking condition until cleanup finishes, after which `status` returns Kubernetes NotFound. Ordinary stack `stop` retains environment intent, namespaces and all database storage; it does not delete boundaries.

## Desired state and validation

The cluster-scoped `ForgeEnvironment` API is `platform.forge.local/v1alpha1`, with CRD name `forgeenvironments.platform.forge.local` and alias `fenv`. Its resource name must be `workload-<UUID v4>`. Spec contains only `workloadID`, `owner.issuer`, `owner.subject` and `profile: small-v1`; all are immutable. API-server validation rejects malformed identities/profiles and mutation. The operator wrapper uses strict field validation so unknown fields fail rather than being silently pruned. Status has a separate subresource; applying intent cannot write observed status.

The controller queries PostgreSQL to verify the exact workload/issuer/subject before provision or repair. `forge_environment` has only connection/schema access and SELECT on workload ID/owner ID and principal ID/issuer/subject columns. It cannot read version/run/command data or names/descriptions, mutate product records, or read the migration ledger. Identity lookup failure preserves existing resources and reports failure; it never adopts another owner's namespace.

The namespace is deterministically `forge-w-<workload UUID>`. Namespace, quota, LimitRange and worker service account reference the exact environment UID. Labels include the workload and an owner digest for bookkeeping. Labels alone grant no permission and cannot substitute for controller ownership references. A cluster-scoped resource can own a namespace; a namespaced custom resource cannot own a cluster-scoped dependent. See [Kubernetes ownership rules](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/).

## Small resource profile

| Boundary | Policy |
|---|---|
| Pod count | 2 |
| Aggregate requests | 1 CPU, 1 GiB memory |
| Aggregate limits | 2 CPUs, 2 GiB memory |
| Default per-container requests | 100m CPU, 64 MiB memory |
| Default per-container limits | 500m CPU, 512 MiB memory |
| Maximum per-container limits | 1 CPU, 1 GiB memory |
| Persistent volume claims / external Services | 0 PVCs, 0 LoadBalancer Services, 0 NodePort Services |
| Runtime admission | Restricted Pod Security, pinned enforce version v1.37 |
| Service accounts | `worker` and Kubernetes-created `default` disable implicit token mounting; no Forge RBAC grants |

Requests/limits are supplied by LimitRange admission and bounded by ResourceQuota. These are local ceilings, not reserved node capacity or a per-owner quota system. They do not cover ephemeral disk or process counts. [LimitRange](https://kubernetes.io/docs/concepts/policy/limit-range/) and [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/) explain admission and existing-Pod limitations.

Restricted admission rejects privileged/host access and requires future worker manifests to supply non-root execution, no privilege escalation, dropped capabilities and a permitted seccomp profile. It does not automatically fill those security contexts. Service-account defaults do not prevent a privileged Kubernetes operator from explicitly requesting a token mount; the account still has no application RBAC grants. See [Pod Security admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/).

## Observation and recovery

Absent status is pending observation. The controller records `Provisioning`, `Ready`, `Failed` or `Deleting`, current `observedGeneration`, a `Ready` condition and transition timestamp, and the observed namespace name/UID. Ready means the namespace is active and boundary resources/default account are reconciled. It is unrelated to worker availability or Temporal execution status. Failure is recoverable when identity lookup or resource conflicts are repaired.

The controller watches intent and owned boundary resources, with a 15-second fallback requeue. Each pass has a ten-second context deadline; the database pool has at most four connections and five-second connection setup. API conflicts retain intent and controller-runtime retries with its bounded workqueue backoff. A leader Lease ensures one active controller. Process health and leader/cache/database readiness are internal Pod probes on port 8084, without a Service or host forward. Metrics are disabled.

Direct reads precede updates so cache lag cannot justify adoption/deletion. Writes preserve unrelated metadata and use resourceVersion conflicts; namespace deletion additionally supplies exact UID/resourceVersion preconditions. Duplicate intent, replay after interruption and missing quota/LimitRange/account children converge to the same boundary. Existing foreign resources produce `OwnershipConflict`; Forge never force-adopts them.

## Cleanup safeguards

The controller installs `platform.forge.local/environment-cleanup` before creating anything. An operator delete leaves that finalizer until the namespace is actually absent. Cleanup verifies the namespace's exact controller ownership, refuses any PVC or PV reference to that namespace, and reports `PersistentStoragePresent`. Quota normally prevents PVC creation; checking existing storage also covers administrator bypass and orphaned PV references.

Kubernetes then deletes namespace contents through its normal controller. Foreign resource/namespace finalizers can leave cleanup pending. Forge never strips them, edits the namespace `/finalize` subresource, removes volumes or bypasses storage protection. Investigate the referenced finalizer owner or retained storage before resolving a block. Removing Forge's finalizer manually or deleting the CRD can bypass this controller's safeguards and is outside ordinary operation. See [Kubernetes finalizers](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/).

Storage inspection and namespace deletion are separate API operations. Coordinate privileged storage changes during cleanup: there is no atomic transaction preventing a cluster administrator from adding a PV reference after inspection. Ordinary PVC admission remains disabled by the environment quota.

`forge-local`, its PostgreSQL PV and infrastructure namespaces cannot be derived from a valid workload identity. Test cleanup verifies they remain. The controller itself is privileged infrastructure: dynamic namespace creation requires cluster permissions that Kubernetes RBAC cannot restrict to a name prefix. Its ClusterRole excludes secrets, Pods, workloads, RBAC mutation and PV writes; code guards constrain namespace operations. This is one trusted engineering-team operator, not isolation from a compromised controller or cluster administrator.

## Limits and follow-up

Kind's current network plugin does not enforce NetworkPolicy; namespace/resource separation makes no network-isolation claim. A later plugin/policy migration needs database retention and connectivity tests. [NetworkPolicy documentation](https://kubernetes.io/docs/concepts/services-networking/network-policies/) explains the enforcement dependency.

There are no worker images, deployments, per-owner quota admission, image allowlist, environment HTTP endpoints, external storage or cloud resources here. Controller dependency and compile/image size grew with the Kubernetes client libraries. Replacing the operator wrapper with an authenticated product API later requires durable intent handoff and its own access design; bypassing the existing fixed research spec is not supported. The existing [command reconciler](command-reconciliation.md) stays separate.

See [F12a/F13 validation](f13-validation.md) for executable recovery/admission/cleanup evidence. Kubernetes performance targets remain unselected; native Balanced thresholds do not transfer automatically.
