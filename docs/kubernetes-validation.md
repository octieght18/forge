# Kubernetes packaging and migration validation

Owner-approved scope: API, PostgreSQL and Keycloak in kind inside WSL; copy existing product/identity data; preserve loopback URLs/issuer; retain native originals. This is current-stack deployment packaging, separate from the amended F12 command reconciler and future workload provisioning/Temporal workers.

## Local results — 8 October 2026

Validated on this PC's existing Ubuntu 26.04 WSL2/systemd environment with approximately 7.6 GiB WSL RAM. Installed Docker Engine 29.8.2/buildx, checksum-verified kind 0.33.0 and kubectl 1.37.0. Used the digest-pinned Kubernetes 1.37.0 node, PostgreSQL 18.6, Keycloak 26.8.0, NGINX 1.28.0 and Go 1.27.1 builder. The API/migration scratch image runs as UID/GID 65532; its first context transfer was approximately 366 kB of allowed source inputs.

| Check | Result |
|---|---|
| Private logical copy | All 6 Forge and 101 Keycloak tables matched source row counts and deterministic row digests before identity startup; database locale/owners/role credentials were preserved. |
| Corrected restore path | Both original custom-format dumps restored into isolated probe databases, with full matching table data; probe databases were removed. Bounded archive transfer plus file-based restore completed. |
| Real identity | Authorization-code/S256 PKCE login passed for Ahmad's existing developer subject, the second developer and operator. Wrong/missing PKCE rejected; password grant remains disabled. |
| Ownership | Existing workload and immutable version readable by owner; second owner reads/mutations and operator mutation rejected; operator inspection succeeds; unauthenticated access rejected. |
| Client compatibility | Windows loopback readiness and exact issuer verified. Published PowerShell client exercised all seven operations, including ETag metadata update and cursor pages, before and after retained restart. Synthetic example workloads/versions were added to the copied database. |
| Retained full restart | Application Pods drained, PostgreSQL stopped cleanly, kind node stopped, then same node/PV restarted. Old workload/version, all three owner subjects and cursor key survived; login/authorization checks passed again. Final API, Keycloak and PostgreSQL Pods started with zero container restarts. |
| Body/error boundary | A 70,000-byte mutation still returns the API's 413 JSON error through the streaming proxy. |
| Import protection | A second import is refused before changing database records; native files and private backups remain present. |
| Static guards | Four tests cover container/sidecar privilege and service exposure, credential separation/exact issuer, immutable dependency locks/retained storage and the source-only build context. Python compilation, shell syntax, documentation and PowerShell checks pass. |

The host still has its original native installation and database files. The running stack uses the independent Kubernetes copies. No dispatcher, Temporal worker, MCP/model call or arbitrary workload container was started. No image was pushed to a registry.

## Corrections and retained evidence

Initial attempts remain in local diagnostic files. Docker's containerd image store could not load incomplete multi-platform indices; single-platform archives preserving image tags fixed both import and CRI name lookup. Direct streaming of a custom archive into pg_restore left a command open after restore; complete transfer to a private temporary file followed by non-streaming restore fixed it. Row digests initially differed because of session timezone/collation; UTC/C normalization and explicit source-locale validation made comparisons portable. NGINX's default temporary paths conflicted with its read-only root; all temporary directories now use /tmp. A normal proxy/API startup order could race OIDC initialization; a native sidecar startup probe now orders initialization and shutdown. The first retained node restart exposed a migration connection race; the migration Job now waits with bounded authenticated SQL probes. The final full stop/start passed with these corrections.

Private dumps, secrets, tokens, row hashes, diagnostic logs and raw F10 profiles remain local. The public repository contains code and this summarized report only. The one-time import does not automatically recover a partially restored target; the guide documents retained-backup investigation rather than overwriting data. Restore size is bounded to 256 MiB per dump.

## Hosted verification and limits

CI now exercises the same native-to-kind database copy with real Keycloak login/owner checks, published PowerShell examples, full kind stop/start and retained identity/product verification in the native integration job. Existing Windows/Linux Go checks, PostgreSQL/race tests and native retained-data checks remain required. Exact commit/job outcomes are linked in the F07a card after completion; this report describes local observations and the checked-in CI protocol.

This is a single-node local development deployment, with Keycloak dev mode/HTTP, administrator-readable Kubernetes Secrets and default kind networking. It does not demonstrate HA, enforced NetworkPolicy, CRD-based provisioning, registry releases or supply-chain scanning/signing. Native Balanced performance thresholds have not been applied to this deployment. See [the operator guide](kubernetes-local.md) for lifecycle, private data and rollback limitations.
