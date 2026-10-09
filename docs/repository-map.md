# Repository navigation

Start with the [quickstart](quickstart.md), [seven current API operations/examples](api-examples.md) and [OpenAPI JSON](../internal/contract/openapi.json). The schema includes planned research-run APIs as well as implemented registration; the examples page identifies availability.

| Path | Purpose |
|---|---|
| [cmd/api](../cmd/api) | API composition, configuration and process lifecycle. |
| [cmd/forge](../cmd/forge) | Developer CLI for register, deploy, status, and delete against the registration API. |
| [cmd/forge-migrate](../cmd/forge-migrate) | Explicit migration command; migrations do not run inside API startup. |
| [cmd/forge-login](../cmd/forge-login) | Five-minute browser/PKCE token helper on loopback 8083. |
| [internal/cli](../internal/cli) | Developer CLI client, local schema checks, JSON results, and API error operation IDs. |
| [internal/httpapi](../internal/httpapi) | Health/registration handlers, owner-filtered pages, ETags, corpus approvals, errors and middleware. |
| [internal/auth](../internal/auth) | Real OIDC discovery/JWKS and API token verification/roles. |
| [internal/login](../internal/login) | State/cookie-bound authorization code exchange and private token file. |
| [internal/contract](../internal/contract) | Shared OpenAPI, examples, schema validation, input fingerprints and run lifecycle consistency tests. |
| [internal/store](../internal/store) | PostgreSQL queries, explicit checked migrations and durable run/start/cancel records. These run records are not HTTP execution endpoints. |
| [internal/reconcile](../internal/reconcile) | F12 command reconciliation and verified backend boundary; no deployed Temporal adapter yet. |
| [internal/environment](../internal/environment), [cmd/forge-environment-controller](../cmd/forge-environment-controller) | Separate operator environment controller with read-only identity lookup, boundary reconciliation and guarded finalization. |
| [internal/config](../internal/config), [internal/service](../internal/service) | Validated configuration, HTTP deadlines, readiness and graceful drain. |
| [internal/testsupport](../internal/testsupport) | Real PostgreSQL integration fixtures. |
| [deploy/native](../deploy/native) | Native PostgreSQL/Keycloak/API lifecycle, private launchers, realm and real-login smoke. |
| [Dockerfile](../Dockerfile), [.dockerignore](../.dockerignore) | Static non-root API/migration image and source-only build context. |
| [deploy/kubernetes](../deploy/kubernetes) | kind resources, fixed proxy, digest lock, retained migration/lifecycle and verified tools. |
| [scripts/kubernetes-stack.ps1](../scripts/kubernetes-stack.ps1) | Windows entry point for copied Kubernetes stack in WSL. |
| [scripts/environment.ps1](../scripts/environment.ps1) | Operator apply/status/delete intent for per-workload Kubernetes boundaries. |
| [scripts/local-stack.ps1](../scripts/local-stack.ps1) | Windows `up`, `status`, `stop`, `login` entry point. |
| [scripts/api-examples.ps1](../scripts/api-examples.ps1) | Runnable seven-operation example with a private access-token file. |
| [scripts/cli-examples.ps1](../scripts/cli-examples.ps1) | Runnable developer CLI register, deploy, status, and rejected-delete example. |
| [scripts/measure-baseline.ps1](../scripts/measure-baseline.ps1) | F08 retained-data startup/registration/recovery measurements. |
| [scripts/profile-summary.py](../scripts/profile-summary.py) | F10 raw request/journey latency and error summary. The opt-in profiling harness lives in `internal/service/profile_test.go`. |
| [.github/workflows/ci.yml](../.github/workflows/ci.yml) | Windows/Linux checks, real PostgreSQL/race/restart and isolated native clean-start/quickstart checks. |

Product intent is in the [brief](product-brief.md), approved choices in the [decision log](decision-log.md), and boundaries/alternatives in [architecture](architecture.md) and [ADRs](adr/README.md). [Persistence](persistence.md), [registration](registration-api.md) and [native setup](local-stack.md) cover operator details. F01–F08 documents retain ticket-stage context; current availability is described in this quickstart/API reference. [F08 baseline](f08-baseline.md) contains observed timings, not performance SLOs. [F09 validation](f09-validation.md) states which clean-checkout paths were actually exercised.
