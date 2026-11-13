# Tenant threat model and failure-mode analysis — F31 / F55

This is a design for the demo platform. [D46](decision-log.md#d46--f31-tenant-threat-model) and [D70](decision-log.md#d70--f55-threat-model-and-failure-modes) record the boundary. **G3 is Done** ([g3-validation.md](g3-validation.md)). **G4 is accepted with limits** ([g4-validation.md](g4-validation.md)): unit/CI and documented controls pass; live NetworkPolicy deny, live Kyverno, and signed-image verify remain undemonstrated. This document does not treat the demo controls as production isolation.

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
| Audit / usage meters | In-process only; lost on process exit |
| Canary release pin | Git overlay files under `deploy/gitops/overlays` |

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

## Threats → controls → tests → gaps

| Threat / failure | Control | Test / evidence | Remaining gap |
|---|---|---|---|
| Spoofed / expired / wrong-audience token | OIDC JWKS verification; no identity headers | `TestAccessTokensAndKeyRotation`; F40 | No revocation before 5-minute lifetime |
| Service token on product routes | Distinct service identity → 403 | F33 | Not a mesh SPIFFE deployment |
| Cross-owner mutation | Role matrix | F34 `TestRoleMatrixAndDeniedTools` | Org multi-tenant still out of scope |
| Cross-tenant read | Tenant/project on rows; other tenant → 404 | F32 | Demo tenant on legacy rows |
| Confused deputy / tool abuse | Only `corpus.search` / `corpus.read`; allow list before process | F23/F34; F47 `tool_model_outage` redaction | No additional tools |
| SSRF via workload URL | Tools are not HTTP clients; model URL fixed in code | Design + code review | Future URL tools would be new threats |
| Worker loss mid-run | Temporal history SoT; replay without re-calling tools | F47 `worker_loss` 38118 µs / 0 side effects; F29 replayer 535 µs | No live process kill demonstrated |
| Tool/model outage | Retryable classes capped at 3; denied not retried | F47 30695 µs; F29 43361 µs | Free-route availability unmeasured in prod |
| Database login loss | Health ≠ store; owner recover; other subject 404 | F47 `database_outage` 17329 µs | Retained DB not stopped; no multi-AZ |
| Quota exhaustion | Per-process caps; 429 Retry-After | F47 `quota_exhaustion` 16572 µs; F35 | Not distributed |
| Bad canary rollout | Pin restore; human gates for migration/model | F47 33 µs; F45 54.4 µs call | Live staging window not collected |
| Backup gaps | Document unrecoverable Temporal/evidence | F46 dump/restore ms; `Encrypted`/`Offsite` false | No offsite encrypted backup |
| Privileged admission bypass | Kyverno policy file + CI checker | F38 | Kyverno not installed live |
| Network lateral movement | Default-deny manifests | F38/F40 | Kind does not enforce NetworkPolicy |
| Supply-chain surprise | Module SBOM; optional scan/sign | F39 | No image signed in session |

## Durability and side-effect safety

- Workflow timeout 2m; activity start-to-close 30s; heartbeat 10s.
- Retryable: `tool_unavailable`, failed configured-model call (3 attempts). Not retryable: `tool_denied`, `model_invalid`, `identity_mismatch`.
- Terminal states: `complete`, `fail_closed`, `canceled`, `timed_out` with `compensation_completed` vs `compensation_incomplete`.
- Replay must not call MCP/model; re-execution is a new run. Duplicate side effects measured **0** in F47 scenarios.
- PostgreSQL is not the execution status authority after dispatch.

## Recovery boundaries

| Domain | Recovery path | Cannot recover |
|---|---|---|
| API process crash | Restart; in-process audit/evidence gone | Lost meters |
| PostgreSQL rows | F46 disposable dump/restore drill | Temporal history, process evidence |
| Worker | Replay history | Unrecorded external effects if any existed outside fixtures |
| Canary pin | F45/F47 rollback | Applied migration; completed model call |
| Cloud cluster | Not created — promotion refused | N/A |

## Tenant isolation limits (demonstrated vs required)

**Demonstrated:** owner-private reads/writes; operator read-only cross-owner; other-tenant 404; service token 403; MCP allow list; secret redaction in tool errors; disposable-DB isolation during F47 outage/quota.

**Required for production (not claimed):** organization isolation; enforcing NetworkPolicy with observed deny; live Kyverno; reviewed image scan; cosign verify against operator key; token revocation; private model route; issued DNS+TLS; distributed quota/audit. G3/G4 gate reviews are closed with documented demo limits and do not satisfy these production items.

## F40 / F55 gap table

| Gap | Why it stays open |
|---|---|
| Default-deny network enforcement | Kind's current network plugin does not enforce NetworkPolicy |
| Live Kyverno admission | ClusterPolicy is a file; Kyverno was not installed |
| Signed image and vulnerability scan | Tools/images absent or not-run; no clean scan claimed |
| Production gateway, TLS, and DNS | Not issued; local API is loopback HTTP |
| Live worker kill / Temporal server failure | F47/F29 use constructed history / SDK tests |
| G3 / G4 live gaps above | Gates reviewed; NetworkPolicy deny, Kyverno install, and cosign still open |

## Before any production deployment

Do not treat this demo as production-ready. A production deployment still requires, and this repository does not have: organization-level isolation, an enforcing network policy with an observed deny, Kyverno admission on the cluster rather than only in CI, a reviewed image scan, a signature that is verified against an operator-held key, token revocation before the five-minute access-token lifetime, a private model route, and an issued DNS name with a live TLS certificate. G3 and G4 owner reviews closed with demo limits; none of the production items above are claimed here.
