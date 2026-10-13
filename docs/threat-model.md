# Tenant threat model and authorization — F31

This is a design for the demo platform. [D46](decision-log.md#d46--f31-tenant-threat-model) records the boundary. G3, the security gate listed as this ticket's prerequisite, is still open. This document does not mark G3 done and does not treat the demo controls as production isolation.

The accepted customer is one engineering team with multiple workload owners. Multiple organizations are outside that decision. Developers read and mutate their own workloads and runs. Operators can read every owner's records and cannot mutate another owner's workload, version, run, or cancel command.

## Assets

| Asset | Where it lives |
|---|---|
| Workload, version, run, and start/cancel command | PostgreSQL, owned by issuer and subject |
| Execution history and workflow result | Temporal, workflow id `forge-run/<run UUID>` |
| Corpus passages | Worker memory and the fixed local snapshot. The API does not return excerpt text |
| Access token | Caller. The API validates it and does not store it |
| `FORGE_MODEL_API_KEY` | Worker environment. Sent only as an Authorization header to the configured model route |
| Cursor signing key | API process |
| Namespace and quota | Environment controller. Developers do not receive a kubeconfig |

## Trust boundaries

| Boundary | Who is inside | Who is outside |
|---|---|---|
| Developer token | One Keycloak subject with role `developer` | Other subjects, and the operator role for mutations |
| Operator token | Role `operator`, read across owners | Mutations of another owner's product records |
| API process | Registration and run routes after token checks | The cluster and the model network |
| Worker | Temporal activities and the two corpus tools | Workload-supplied commands, URLs, and extra tools |
| MCP child | `PATH` only, started after the document allow list check | The worker's other environment variables |
| Environment controller | Privileged namespace apply for an open provision operation | Developer credentials. Its database role is read-only |
| Keycloak | Issuer keys and access tokens | Forge product policy |
| OpenRouter | The free Gemma route when a key is configured | Debug runs, which use the fixture instead |

A tenant in this design is the engineering team. A project is a group of workloads inside that team. F32 stores those identifiers. Rows that existed with only issuer and subject keep the demo tenant and demo project. They are not deleted. This is still one team, not a boundary between organizations.

## Roles

| Action | Developer | Operator |
|---|---|---|
| Create and patch own workload, version, and run | Yes | Only for records that subject owns |
| Read own workload, version, run, history, and result | Yes | Yes, including other owners |
| Cancel or rerun own run | Yes | No, when the owner is someone else |
| Read telemetry | No | Yes |
| Apply a namespace | No | Through the controller and `scripts/environment.ps1`, not a kubeconfig handed to a developer |

## Threats

**Spoofing.** A caller presents a token with a forged signature, the wrong issuer, the wrong audience, or an expired time, or omits the developer and operator roles. `TestAccessTokensAndKeyRotation` rejects those tokens for the registration verifier. Run routes use the same verifier. The API does not accept identity headers.

**Confused deputy.** A workload must not choose the MCP command, a URL, or a tool name outside `corpus.search` and `corpus.read`. The worker starts one configured executable only after the frozen document allow list matches. The environment controller is privileged infrastructure: it can create a namespace, and a developer cannot steer that privilege with a kubeconfig. The controller's database role cannot write product rows.

**Cross-tenant data access.** F32 stores a tenant and a project. A developer or operator read of another tenant's id returns 404. Operators can read other owners inside the same tenant and project. Existing rows keep the demo tenant. That is still one team, not a boundary between organizations. An operator read includes the submitted question because the Run schema requires it. History and result omit the question and corpus text.

**SSRF.** The research tools are not HTTP clients. The model route used by a configured worker is the OpenRouter endpoint in code, not a URL from the workload. A workload field cannot name a host. Debug mode does not call that network. A future tool that accepted a URL would be a new threat and is not part of this design.

**Tool abuse.** The version permits exactly the two corpus tools. A document outside the frozen list fails before a process starts. The child keeps `PATH` and does not inherit the worker environment. A denied tool in the failed-step evaluation does not call the tool function.

## Controls

Tested in this repository: Keycloak access-token checks for issuer, audience, signature, expiry, and role; owner reads and operator read-only mutation checks on workloads, versions, runs, cancel, history, and rerun; MCP allow-list rejection before a process; fixture debug path that does not use a live model key. F35 adds a per-process request limit and a database cap on workloads and pending runs. F36 records an audit event without the token. F37 names secret homes and refuses demo cleanup of the retained database.

The stored tenant is still the demo tenant for existing rows, not an organization boundary. Service and workload identities are not a production mesh. The request limit does not leave the API process. Audit events leave with the process. G3 remains open. G4 is not marked done in this repository. The environment controller is trusted infrastructure, not a tenant sandbox.

## F40 gaps

These checks were not demonstrated:

| Gap | Why it stays open |
|---|---|
| Default-deny network enforcement | Kind's current network plugin does not enforce NetworkPolicy. No connection was attempted, so no denied path was observed |
| Live Kyverno admission | The ClusterPolicy is a file. The CI checker rejected a privileged fixture. Kyverno was not installed |
| Signed image and vulnerability scan | `syft`, `grype`, `trivy`, and `cosign` were absent. No clean scan is claimed |
| Production gateway, TLS, and DNS | Not issued. The local API is loopback HTTP |

## Before any production deployment

Do not treat this demo as production-ready. A production deployment still requires, and this repository does not have: G3 closed, G4 closed, organization-level isolation, an enforcing network policy with an observed deny, Kyverno admission on the cluster rather than only in CI, a reviewed image scan, a signature that is verified against an operator-held key, token revocation before the five-minute access-token lifetime, a private model route, and an issued DNS name with a live TLS certificate. None of those are claimed here.
