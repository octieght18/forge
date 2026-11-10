# Architecture diagrams — F52

[D67](../decision-log.md#d67--f52-architecture-diagrams-and-adr-index) records the boundary. These diagrams describe the **implemented demo composition**. They do not close G3–G6 or claim a live cloud deployment.

Companion: [ADR index](../adr/README.md) · [architecture narrative](../architecture.md) · [threat model](../threat-model.md)

## 1. Personas and primary paths

```mermaid
flowchart TB
  subgraph personas [Personas]
    Dev[Developer]
    Ops[Operator]
  end
  Dev -->|PKCE login| KC[Keycloak]
  Ops -->|PKCE login| KC
  Dev -->|Bearer user token| API[Go HTTP API]
  Ops -->|Bearer user token| API
  Dev -->|register deploy provision CLI| CLI[forge CLI]
  CLI --> API
  Ops -->|environment.ps1 apply| CTRL[Environment controller]
  Ops -->|read telemetry audit| API
  Dev -.->|no kubeconfig| DenyKube[Denied]
```

## 2. Identity, API, and product store

```mermaid
flowchart LR
  Token[Access token] --> Verifier[OIDC JWKS verifier]
  Verifier -->|developer or operator| API[Go HTTP API /api/v1]
  Verifier -->|service token| S403[403 product routes]
  API --> PG[(PostgreSQL product DB)]
  PG -->|workloads versions runs start/cancel| Rows[Issuer subject tenant project]
  API -->|other tenant or other owner miss| H404[404]
  API -->|rate or quota| H429[429 Retry-After 1]
```

## 3. Provisioning and controller

```mermaid
sequenceDiagram
  participant Dev as Developer
  participant API as API
  participant PG as PostgreSQL
  participant Ops as Operator
  participant Ctrl as Environment controller
  participant K8s as Kubernetes API
  Dev->>API: POST environments
  API->>PG: operation accepted
  API-->>Dev: 202 status URL
  Ops->>Ctrl: apply ForgeEnvironment
  Ctrl->>PG: read-only identity lookup
  Ctrl->>K8s: reconcile namespace quota SA
  Note over Ctrl,PG: Controller does not write operation status
  Dev->>API: GET operation status
  API->>PG: compare observed phases
```

## 4. Asynchronous execution data flow

```mermaid
flowchart TB
  API[API] -->|txn: run + start command| PG[(PostgreSQL)]
  Disp[Dispatcher] --> PG
  Disp -->|workflow id forge-run/UUID| Temporal[Temporal]
  Worker[forge-worker] --> Temporal
  Worker -->|activities| MCP[corpus.search / corpus.read]
  Worker -->|fixture or free Gemma| Model[OpenRouter free route]
  Worker -->|evidence in process| Evidence[In-process evidence meter]
  Temporal -->|history SoT| Hist[Execution history]
  API -->|history result reads| Temporal
  API -->|run and commands only| PG
```

## 5. Trust boundaries

```mermaid
flowchart TB
  subgraph outside [Outside]
    Internet[Internet clients]
    OR[OpenRouter]
  end
  subgraph idb [Identity boundary]
    KC[Keycloak]
  end
  subgraph apib [API process]
    HTTP[HTTP handlers]
    Audit[In-process audit]
  end
  subgraph datab [Data plane]
    PG[(Product PostgreSQL)]
    TDB[(Temporal persistence)]
  end
  subgraph execb [Execution boundary]
    Worker[Worker]
    MCP[MCP child PATH only]
  end
  subgraph opsb [Operator infrastructure]
    Ctrl[Environment controller]
    K8s[Cluster]
  end
  Internet --> HTTP
  HTTP --> KC
  HTTP --> PG
  Worker --> TDB
  Worker --> MCP
  Worker --> OR
  Ctrl --> K8s
  Ctrl -.->|read-only| PG
```

## 6. Failure domains

```mermaid
flowchart LR
  FD1[API process<br/>rate limit audit meters] 
  FD2[PostgreSQL product<br/>login loss 503 then recover]
  FD3[Temporal + worker<br/>history replay no duplicate tools]
  FD4[Tool or model fixture<br/>retryable vs denied]
  FD5[Delivery pin<br/>canary rollback]
  FD6[Backup dump<br/>no Temporal history]
  FD1 --- FD2
  FD2 --- FD3
  FD3 --- FD4
  FD5 --- FD6
```

Measured samples (not SLOs): F47 worker_loss 38118 µs / 0 side effects; tool_model_outage 30695 µs / 1 evidence; database_outage 17329 µs; quota_exhaustion 16572 µs; bad_rollout 33 µs; F46 dump 61.76 ms / restore 71.79 ms; F45 rollback call 54.4 µs.

## 7. Telemetry

```mermaid
flowchart LR
  Req[Request] --> API[API OTel counters]
  Op[Provision operation] --> API
  Act[Activity] --> Worker[Worker meters]
  Worker -->|tokens once per idempotency key| Usage[In-process usage]
  API -->|actor tenant path outcome| Audit[In-process audit]
  API -->|operator read| View[Telemetry HTTP view]
  Note1[No collector or dashboard shipped]
```
