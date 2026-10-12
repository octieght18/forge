# Authorization matrix — F34

Permission is checked at the API, at provisioning, and before a tool process starts. [D49](decision-log.md#d49--f34-rbac-and-tool-authorization) records the boundary. Tenant and project scope comes from F32. User, service, and workload identity come from F33.

| Action | Developer | Operator | Service |
|---|---|---|---|
| Create or patch a workload the subject owns, in the caller's tenant and project | Yes | Yes, only for that subject | No, 403 |
| Read a workload, version, run, history, or result in the same tenant and project | Own records | Any owner | No, 403 |
| Read or mutate another tenant | 404 | 404 | No, 403 |
| Mutate another owner's workload, version, run, cancel, or rerun | 404 | 404 | No, 403 |
| Accept a provision operation | Own workload | 403 | No, 403 |
| Read telemetry | 403 | Yes | No, 403 |
| Call `corpus.search` or `corpus.read` inside the frozen document list | The worker, for that run | Not a token action | Not a token action |
| Any other tool name | No process | No process | No process |

A token that mixes `developer` or `operator` with `service` is rejected. A token that claims `identity_kind=workload` is rejected. The workload identity remains the stored run.

The tool check happens before `exec`. `corpus.write` returns denied and does not start the configured command. A document outside the frozen list fails in the same way: the tool function is not called. The activity input does not carry a second tenant lookup. The run was admitted only when the API's tenant and project matched, and the worker does not add a tool the version did not name.

G3 stays open. This matrix is the demo's enforced rules, not a production isolation claim.
