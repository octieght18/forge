# Self-service contract — F20

A developer asks for an environment. The controller converges the cluster to that request. [D34](decision-log.md#d34--f20-self-service-contract) records this boundary. The request, the environment object, and the namespace are three records with one workload identity.

Declarative state is the requested action and the `ForgeEnvironment` spec. Idempotency means a repeated request and a repeated reconcile leave one boundary. Ownership means a resource is changed only when its controller reference is this environment.

## API to controller

1. `forge template render` writes a local service, agent, or job. The files use `research.v1`. The job records profile `small-v1`.
2. `register` creates the workload. `deploy` stores an immutable version. The version reports `execution: not_started` and `environment: not_requested`.
3. `provision` accepts an operation and returns `202` with `status_url`. The body selects the current `version_id`. One open operation is allowed for that workload.
4. An operator runs `environment.ps1 apply`. That command reads the workload through the product API and submits immutable intent: workload ID, owner issuer, owner subject, and `profile: small-v1`.
5. The controller reads that intent, checks the same owner in PostgreSQL, and creates namespace `forge-w-<workload UUID>`, the `boundary` quota, the `containers` limit range, and the `worker` account.
6. Status on the environment object moves through the phases below. Operators read it with `environment.ps1 status`.

The operation row and the environment status use the same phase comparison. The installed controller's database role can read identity and cannot write the operation row. A live namespace can be `Ready` while the operation remains `accepted` until a deadline or a recorded observation. Tests record observations through that comparison. [Provisioning operations](provisioning-operations.md) and [execution boundaries](execution-environments.md) are the command references.

## Lifecycle

Developer operation:

| State | What moves it |
|---|---|
| `accepted` | `POST` provision or operation delete |
| `provisioning` | An observed `Provisioning` phase during provision, or `Deleting` during a delete that was not a cancel |
| `ready` | Observed `Ready` during provision. Observed generation matches desired generation |
| `cancel_requested` | Cancel of an open operation. The action becomes delete |
| `canceled` | `Deleting` was already observed, then `Absent` |
| `failed` | `OwnershipConflict`, or phase `Failed` |
| `timed_out` | The deadline passes before the desired phase is observed |

`Ready` during a delete does not finish the operation. `Absent` before `Deleting` does not finish it. A finished operation stays finished when a later phase arrives. A boundary that becomes ready after a `failed` operation needs a new operation.

Controller boundary:

| Phase | Reason | Meaning |
|---|---|---|
| `Provisioning` | `Reconciling` | Owner matched. Desired children are being created or repaired |
| `Provisioning` | `NamespacePending` | The namespace exists and is not active |
| `Provisioning` | `DefaultAccountPending` | The Kubernetes `default` account is not present yet |
| `Ready` | `BoundaryReady` | The namespace is active, children match, and the default account does not automount a token |
| `Failed` | `IdentityUnavailable` | The product lookup returned an error. No namespace is created for a new boundary |
| `Failed` | `OwnerMismatch` | The issuer and subject do not match the workload |
| `Failed` | `InvalidIdentity` | The intent cannot be decoded |
| `Failed` | `OwnershipConflict` or `ResourceUnavailable` | A child cannot be created or repaired without taking over a foreign object |
| `Deleting` | `CleanupPending` | Namespace deletion was requested and the finalizer remains |
| `Deleting` | `PersistentStoragePresent` | A PVC or a PV claim reference is still in the namespace. The namespace stays |
| `Deleting` | `OwnershipConflict` | The namespace controller reference is not this environment |
| `Absent` | `CleanupComplete` | The namespace is gone and Forge's finalizer is removed |

`Ready` means the boundary resources exist. It does not mean a worker is running or a research run finished.

## Declarative state, idempotency, and ownership

The spec is the desired boundary. Status is what the controller last observed. A developer changes the request by accepting a new operation. An operator changes intent by applying the environment command again. Neither path edits the namespace by hand.

`apply` on an existing workload keeps the same custom resource and the same namespace. A reconcile that finds the quota, limit range, and worker account already correct writes nothing. A second pass after `BoundaryReady` keeps the environment resource version. Missing children are created again. Drift in the quota or the worker token default is repaired from the same desired objects.

Every managed object carries a controller owner reference to the environment UID, plus the workload label and an owner digest. The digest is bookkeeping. Permission comes from the owner reference and from the controller's checks. A namespace or account that already exists without that reference is left unchanged and reported as `OwnershipConflict`.

## Isolation

| Choice | What this release does |
|---|---|
| Workload boundary | One `ForgeEnvironment` per workload. The name is `workload-<UUID>`. The spec is immutable |
| Namespace isolation | One namespace, `forge-w-<UUID>`, with quota, a limit range, restricted Pod Security, and no implicit service-account token |
| Stronger isolation | A separate cluster, a sandbox runtime, or an enforced NetworkPolicy is a later decision |

The namespace is the isolation boundary this controller can keep idempotent: a deterministic name, one owner UID, and the `small-v1` ceilings (2 pods, 1 CPU and 1 GiB requested, 2 CPU and 2 GiB limited, no PVCs, no LoadBalancer, no NodePort). Those ceilings are admission limits on one shared cluster. Kind's current network plugin does not enforce NetworkPolicy. The controller is privileged infrastructure because creating namespaces requires cluster scope. Its ClusterRole excludes Secrets, Pods, workload objects, RBAC changes, and PV writes. A compromised controller or a cluster administrator is outside this boundary.

Sharing a cluster keeps one control loop and one owner check. A namespace per workload keeps a noisy neighbor inside `small-v1` and gives cleanup a single object to delete. Stronger isolation would separate kernels or networks and would need its own retention and connectivity evidence. This release keeps the namespace boundary.

## Cleanup

The finalizer `platform.forge.local/environment-cleanup` is added before any namespace exists. Operator delete sets the environment deletion timestamp and leaves the finalizer in place. The controller deletes the namespace only when it owns that namespace and no PVC or PV claim reference remains. It uses the namespace UID and resource version as preconditions. Foreign finalizers stay. Forge does not clear another controller's finalizer or the namespace `/finalize` subresource.

When the namespace is gone, Forge removes its own finalizer and reports `Absent`. The product workload and versions remain. Product deletion is rejected by the API. `forge-local` and the retained PostgreSQL volume are not names derived from a workload ID, and cleanup tests keep them.

Storage inspection and namespace deletion are separate API calls. A cluster administrator can add a volume reference between them. The quota still rejects ordinary PVC creation.

## Recovery examples

These are in-process observations of the current controller. They are samples, not a capacity target.

- Identity lookup fails. Two passes report `IdentityUnavailable` and create no namespace. After the lookup succeeds, a new reconciler continues. Deleting the quota and limit range, then reconciling again, restores one owned namespace, one quota, one limit range, and one worker account. The next pass keeps the environment resource version and the namespace UID. One published run measured 7.1103 ms with 1 failure and 2 successes. A later run of the same test measured 9.4041 ms with the same counts. [F18](f18-validation.md) is that test.
- An operation that observed `Failed` stays `failed` after the boundary is ready. A later provision operation can reach `ready` when `Ready` is observed.
- Three template kinds accepted provisioning in 3.258–5.304 ms on a disposable database. That is the request, before a namespace exists. [F19](f19-validation.md) records the three workloads.

## Known limits

- Developers receive no kubeconfig and no namespace RBAC grant.
- The live controller does not write operation rows. Operator status is the environment object.
- A `failed` or otherwise finished operation does not reopen.
- `small-v1` is the only profile. It is a local ceiling, not reserved node capacity or a per-owner quota.
- Run submission, worker images, and Temporal execution are outside this contract.
- The ten paired manual-versus-self-service trials from the baseline procedure are still unclaimed.

See [F20 verification](f20-validation.md).
