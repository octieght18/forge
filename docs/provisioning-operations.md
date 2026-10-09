# Provisioning operations — F16

An accepted provisioning operation returns before the environment controller finishes. The response is `202 Accepted` and includes `status_url`. Poll that path for desired generation, observed phase, and a terminal error when the operation finishes.

This does not create a namespace, grant Kubernetes access, or start a research run. Operators still use [environment apply, status, and delete](execution-environments.md). [D30](decision-log.md#d30--f16-asynchronous-provisioning-operations) records that split.

## Commands

Build the CLI, register a workload, and deploy a version as in the [developer CLI guide](developer-cli.md). Then accept an operation for that current version:

```powershell
$accepted = & $forge provision --token-file $tokenFile --workload $workloadId --version $versionId | ConvertFrom-Json
& $forge operation status --token-file $tokenFile --operation $accepted.provisioning.operation_id
```

`operation cancel` asks for deletion of an operation that is still open. `operation delete` accepts a new delete operation for the current version. A finished operation cannot be canceled. Timeout is 1–120 seconds and defaults to 30. The API base stays `http://127.0.0.1:8081` unless `--api` selects another loopback URL.

## Conflict, cancel, and timeout

| Condition | Result |
|---|---|
| A second operation while one is `accepted`, `provisioning`, or `cancel_requested` | `409 conflict`. The open row is unchanged. |
| `version_id` is not the workload's latest version | `409 conflict`. Nothing is inserted. |
| Cancel of `ready`, `failed`, `canceled`, or `timed_out` | `409 conflict`. |
| Deadline passes before the desired phase is observed | Status becomes `timed_out` with `error.code` `operation_timeout`. |
| Controller reports `OwnershipConflict` | Status becomes `failed` with `ownership_conflict`. |
| Controller reports `Ready` for a provision | Status becomes `ready` and the observed generation matches the desired generation. |
| Controller reports `Deleting` and then `Absent` for a delete or cancel | Status becomes `canceled` with a terminal message. `Absent` before `Deleting` does not finish the operation. |

Owners create, cancel, and read their operations. Another owner receives `404`. An operator can read every operation and cannot create or cancel one. The product workload and version remain after cancel, delete, failure, or timeout.

## Desired and observed state

The operation stores the requested action and desired generation. Controller phases move the observed phase. `Ready` completes only a provision. `Absent` completes a delete only after `Deleting` was observed. A finished operation does not reopen when a later phase or a passed deadline arrives. A matching `Ready` observation at the deadline completes the provision instead of timing out.

The controller emits `Provisioning`, `Ready`, `Failed`, `Deleting`, and `Absent` through its reporter. The installed controller role can only read workload identity, so it does not write this table. Tests record those phases through the same comparison the API uses on read. Live cluster status remains the environment object's status until that write path exists.

See [F16 verification](f16-validation.md).
