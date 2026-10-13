# Forge

A proposed self-service control plane for asynchronous AI workloads.

Forge's purpose is to show how an engineering team can compose existing infrastructure into a platform that developers consume: register a workload, request a controlled execution environment, run a durable workflow, and inspect the result and its evidence.

## Current work

Start with the [local quickstart](docs/quickstart.md), [current API examples](docs/api-examples.md) and [repository map](docs/repository-map.md). These cover native startup, real login, loading a Bearer token into a client and all seven available product operations.

**F01/F02** establish the product and architecture; **F03** supplies the HTTP foundation; **F04/F05** add the contract and PostgreSQL persistence. **F06** implements authenticated workload/version registration, owner checks, ETag updates, signed cursor pages and corpus approvals. **F07** deploys PostgreSQL, Keycloak and the API natively in Ubuntu WSL, with private retained data and real browser/PKCE login. **F08** captures repeatable startup, registration and occupied-port recovery observations with raw evidence and an unmeasured human worksheet. **F09** publishes the first native quickstart, API examples and repository navigation with clean-checkout verification. **F10** validates cancellation/rollback/draining across dependencies, corrects disconnect handling during JWKS waits, and publishes reduced registration CPU/heap profiles and latency/error observations. Run HTTP endpoints and Temporal delivery follow later tickets.

The accepted first release is the **API and durable research workflow only**: one engineering team, owner-private workloads/runs, operator visibility, and a fixed local document corpus accessed through read-only MCP tools. Available project capacity is five hours/week. Environment provisioning and broader platform capabilities follow in later releases.

**F11** defines the fixed research specification and submission/cancellation/execution lifecycle, with executable response consistency checks and ambiguity/race examples. Its original Kubernetes environment checklist is deferred by owner-approved scope amendment; run endpoints and dispatch remain later work.

**F12** supplies PostgreSQL-backed command reconciliation with lease fencing, persisted bounded retries, start-before-cancel and a separate dispatcher role. Native startup prepares its storage/credentials; the Temporal adapter and dispatcher service remain F22 work.

The owner approved [containerized Kubernetes deployment in WSL](docs/kubernetes-local.md): API, PostgreSQL and Keycloak in kind, with copied existing databases and retained native originals. Public loopback URLs and owner identities stay the same. **F12a/F13** restore the environment CRD/controller and provision [one resource/runtime boundary per workload](docs/execution-environments.md), with replay, drift repair and protected cleanup. Research workers and enforced network isolation remain future work.

**F14** adds a [developer CLI](docs/developer-cli.md) over the existing registration API. `register` creates a workload, `deploy` records an immutable version, `status` reads those records, and `delete` reports the API's rejection of deletion. It does not provision an environment or start a run.

**F15** adds [service and MCP agent templates](docs/workload-templates.md) through `forge template`. Both use the fixed research contract. Rendering writes local files only.

**F16** accepts [asynchronous provisioning operations](docs/provisioning-operations.md). A request returns promptly with a status URL. The environment controller still applies namespaces only through the operator command.

**F17** records [in-process OpenTelemetry measurements](docs/telemetry.md) for provisioning duration, reconciliation retries, and request and operation identity. Operators can read the API view. There is no collector or dashboard.

**F18** tests [controller failure recovery](docs/f18-validation.md). An identity-lookup outage, a replacement controller, and a partial boundary converge to one owned namespace. A failed operation stays closed.

**F19** measures [three template onboardings](docs/f19-validation.md): a research service, a tool-calling agent, and a resource-constrained job. Each can be registered, deployed, and requested for provisioning. Namespace apply stays with the operator.

**F20** publishes the [self-service contract](docs/self-service-contract.md): the path from a provision request to the controller, the namespace isolation choice, and the cleanup and recovery limits.

**F21** defines the [execution, tool, and evidence contract](docs/execution-contract.md). Workflow decisions stay replayable. Model and corpus calls stay in activities with typed failures. Run routes stay unimplemented.

**G2** records the [measured self-service gate](docs/g2-validation.md). A developer provision is the environment request. The controller submits that environment and reconciles it. The developer does not receive a kubeconfig.

**F22** runs the research decisions on a [local Temporal worker](docs/temporal-worker.md). PostgreSQL keeps the run and the command. Temporal keeps execution history. The worker does not call a model.

**F23** adds a [read-only MCP adapter](docs/mcp-adapter.md). The worker checks the frozen document scope before it starts `corpus.search`. A deterministic fixture returns one synthetic passage.

**F24** produces an [evidence-based report](docs/model-execution.md) from those passages. Tests use a fixture model. The hosted route is OpenRouter's free Gemma model, and the API key stays in the worker environment.

**F25** classifies [activity retries and compensation](docs/retry-compensation.md). A transient tool failure can run again. A committed retrieval is not repeated. Evidence left without a report is marked compensated.

**F26** adds [timeouts, heartbeats, and cancellation](docs/execution-timeouts.md). Canceling a workflow stops the tool through the activity context and records `canceled`. If compensation does not finish, the record says so.

**F27** exposes [authenticated run history](docs/execution-history.md). Start and cancel commit the existing command records. History and result read Temporal and omit prompts, corpus text, and tokens. Full tenant RBAC remains the M4 gate.

**F28** separates [replay, retry, and re-execution](docs/replay-modes.md). Replay does not call the model or MCP. Retry stays the bounded activity policy. Re-execution is a new run linked to the original. Debug mode uses the fixture tool and fixture model.

**F29** measures [SDK recovery](docs/f29-validation.md): two retrieval attempts after a dependency failure, one compensated evidence row, an uncertain result with no evidence, and a canceled run. A replayer does not repeat the tool. No live worker was killed.

**F30** publishes the [workflow and replay guide](docs/workflow-replay-guide.md), including that sample, the retry and compensation rules, and the limits of the SDK clock. It does not set a production SLO.

**F31** writes the [tenant threat model](docs/threat-model.md). G3 stays open. Keycloak, owner isolation, and the read-only MCP command are existing controls, not production isolation. The environment controller remains privileged infrastructure.

**F32** stores a demo tenant and project on existing rows and scopes workload, version, and run queries. Another tenant's id is 404. Rows are not deleted.

**F33** keeps Keycloak user-token checks and separates a [service identity and a workload identity](docs/identity.md). A service token cannot call product routes. A workload id comes from the run, not from a bearer token. Key rotation stays the existing JWKS test.

**F34** enforces the [role matrix](docs/authorization.md) at the API, provisioning, and tool start. Operators cannot mutate another owner's workload. A denied tool does not start a process.

**F35** limits requests, workloads, and pending runs per tenant. A limited request is 429 with `Retry-After`, not a stack trace. The request window is one process. `small-v1` still caps the namespace.

**F36** records an audit event for an authenticated request and meters fixture prompt tokens, completion tokens, and tool calls once per activity idempotency key. Tokens, prompts, and corpus text are not stored.

**F37** defines [retention and secret homes](docs/retention.md). Executions stay in PostgreSQL and Temporal. Evidence and audit stay in the process. The demo cleanup removes only `demo-synthetic-*` files and refuses the retained database and secret files.

**F38** selects Kyverno and checks [privileged manifests in CI](docs/policy.md). Default-deny network manifests allow same-namespace traffic, DNS, and TCP 443 for the model route. Kind does not enforce NetworkPolicy here, and no denied path was observed.

**F39** writes a Go module [SBOM](docs/supply-chain.md) from `go.sum`. Image scanning and `cosign` run only when the tool and the image exist. A signature does not prove safety. This session did not sign an image or claim a clean scan.

**F40** publishes the [security results that ran](docs/f40-validation.md). Auth, RBAC, quota, and secret redaction passed on a disposable database. Network denial was not observed. G3 and G4 stay open. The demo is not production-ready.

**F41** adds an [EKS Terraform module](docs/terraform-eks.md) with pinned providers. It was validated and not applied. No cloud resources were created and no spend occurred. G4 stays open. GKE is not a second target.

**F42** adds an [Envoy gateway manifest](docs/gateway.md) with `/api/` routing and a local rate limit. It was not applied. DNS, TLS, and a load balancer were not issued. Unauthenticated API requests still return 401.

**F43** adds [Argo CD applications](docs/gitops.md) for local and staging overlays. Image digests are copied only where `images.json` already has one. No signature was invented. Bootstrap credentials stay with the operator. Argo CD was not installed.

- [Internal product brief](docs/product-brief.md)
- [Product and design decisions](docs/decision-log.md)
- [Architecture and integration boundaries](docs/architecture.md)
- [Architecture decision records](docs/adr/README.md)
- [F02 documentation validation](docs/f02-validation.md)
- [Go service setup and conventions](docs/service-foundation.md)
- [F03 validation](docs/f03-validation.md)
- [API contract and conventions](docs/api-contract.md)
- [OpenAPI 3.1.1](internal/contract/openapi.json)
- [F04 validation](docs/f04-validation.md)
- [PostgreSQL persistence and migration guide](docs/persistence.md)
- [F05 validation](docs/f05-validation.md)
- [Authenticated registration setup and behavior](docs/registration-api.md)
- [F06 validation](docs/f06-validation.md)
- [Native local startup, login and retained data](docs/local-stack.md)
- [F07 validation](docs/f07-validation.md)
- [Baseline procedure and measurement commands](docs/baseline-procedure.md)
- [F08 observed timings and raw evidence](docs/f08-baseline.md)
- [F09 quickstart validation](docs/f09-validation.md)
- [F10 cancellation validation and profiling results](docs/f10-validation.md)
- [Reproducible registration profiling procedure](docs/profile-procedure.md)
- [Accepted Balanced local performance targets](docs/performance-targets.md)
- [Research specification and lifecycle](docs/workload-lifecycle.md)
- [F11 verification](docs/f11-validation.md)
- [Durable command reconciliation and upgrade](docs/command-reconciliation.md)
- [F12 verification](docs/f12-validation.md)
- [Kubernetes migration and local commands](docs/kubernetes-local.md)
- [Kubernetes verification](docs/kubernetes-validation.md)
- [Operator environment provisioning](docs/execution-environments.md)
- [F12a/F13 verification](docs/f13-validation.md)
- [Developer CLI](docs/developer-cli.md)
- [F14 verification](docs/f14-validation.md)
- [Workload templates](docs/workload-templates.md)
- [F15 verification](docs/f15-validation.md)
- [Provisioning operations](docs/provisioning-operations.md)
- [F16 verification](docs/f16-validation.md)
- [Telemetry](docs/telemetry.md)
- [F17 verification](docs/f17-validation.md)
- [F18 controller recovery verification](docs/f18-validation.md)
- [F19 onboarding measurement](docs/f19-validation.md)
- [Self-service contract](docs/self-service-contract.md)
- [F20 verification](docs/f20-validation.md)
- [Execution, tool, and evidence contract](docs/execution-contract.md)
- [F21 verification](docs/f21-validation.md)
- [G2 self-service gate](docs/g2-validation.md)
- [Temporal worker](docs/temporal-worker.md)
- [F22 verification](docs/f22-validation.md)
- [MCP tool adapter](docs/mcp-adapter.md)
- [F23 verification](docs/f23-validation.md)
- [Model execution](docs/model-execution.md)
- [F24 verification](docs/f24-validation.md)
- [Retry classification and compensation](docs/retry-compensation.md)
- [F25 verification](docs/f25-validation.md)
- [Execution timeouts and cancellation](docs/execution-timeouts.md)
- [F26 verification](docs/f26-validation.md)
- [Execution history and result](docs/execution-history.md)
- [F27 verification](docs/f27-validation.md)
- [Replay, retry, and re-execution](docs/replay-modes.md)
- [F28 verification](docs/f28-validation.md)
- [F29 recovery measurements](docs/f29-validation.md)
- [Workflow and replay guide](docs/workflow-replay-guide.md)
- [F30 verification](docs/f30-validation.md)
- [Tenant threat model](docs/threat-model.md)
- [F31 verification](docs/f31-validation.md)
- [F32 tenant boundary verification](docs/f32-validation.md)
- [Identity](docs/identity.md)
- [F33 verification](docs/f33-validation.md)
- [Authorization matrix](docs/authorization.md)
- [F34 verification](docs/f34-validation.md)
- [F35 quota verification](docs/f35-validation.md)
- [F36 audit and metering verification](docs/f36-validation.md)
- [Retention and secret homes](docs/retention.md)
- [F37 verification](docs/f37-validation.md)
- [Admission and network policy](docs/policy.md)
- [F38 verification](docs/f38-validation.md)
- [SBOM, scan, and signature](docs/supply-chain.md)
- [F39 verification](docs/f39-validation.md)
- [F40 security validation](docs/f40-validation.md)
- [EKS Terraform plan](docs/terraform-eks.md)
- [F41 verification](docs/f41-validation.md)
- [Gateway, DNS, and TLS](docs/gateway.md)
- [F42 verification](docs/f42-validation.md)
- [Argo CD GitOps](docs/gitops.md)
- [F43 verification](docs/f43-validation.md)

The owner must review product and design choices before they become accepted decisions. F02 records the accepted Go HTTP/OpenAPI, PostgreSQL and Temporal foundation, OpenRouter backend and local Kubernetes target. Keycloak OIDC is included from the first release. The testing model is `google/gemma-4-26b-a4b-it:free`. D26 supersedes F07's no-container deployment choice: the current stack can now run in kind inside WSL. F03's HTTP defaults and lifecycle conventions are accepted; new API/identity/workflow decisions still require consultation.

## Run the foundation

With Go 1.27.1 installed:

```sh
go run ./cmd/api
```

This default command serves process health only. For authenticated registration with the native database and issuer, run `scripts/local-stack.ps1 up` after the documented prerequisites. Read the [native stack guide](docs/local-stack.md) for startup/login/data locations and the [service guide](docs/service-foundation.md) for configuration and tests.

## Delivery plan

1. Product requirements and Go API foundation.
2. Kubernetes self-service provisioning.
3. Durable AI workflows and safe debugging.
4. Tenant security, policy, quotas, audit, and metering.
5. GitOps deployment and operations.
6. Measured evidence, documentation, and portfolio.

The Wekan project contains the original full execution backlog. Its dates and phase assignments need revision to reflect the accepted smaller first release and five project hours/week. Read the brief for the current scope.
