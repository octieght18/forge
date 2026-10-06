# ADR 0005 — Native local deployment

Status: accepted by the owner on 6 October 2026 (D20).

## Decision

Run PostgreSQL, Keycloak and the Go API as native unprivileged services inside the existing Ubuntu WSL2 installation, managed by systemd through a Windows PowerShell entry point. The owner explicitly requested no containers and accepted WSL as the native deployment location. This supersedes F07's original Kubernetes deployment and F02's Compose-first local runtime choice; the Go/OpenAPI, PostgreSQL, OIDC and Temporal integration boundaries remain accepted.

Use a dedicated PostgreSQL cluster and separate product/identity databases and roles. Generate private local credentials and keys, preserve them with data outside Git, and run migrations explicitly before API startup. Use actual Keycloak authorization-code/S256 PKCE login and the F06 access-token policy. Normal teardown retains data; deletion is a separate guarded operator command. Verify Windows loopback connectivity, real ownership enforcement and retained identities/records across orderly restart.

## Alternatives and reversibility

Docker Compose would package infrastructure with less host dependency management, but conflicts with the owner's no-container instruction. Native Windows Java/database installations would avoid the WSL lifetime boundary, but add separate Windows packaging/service-management work. The owner selected the existing Ubuntu WSL environment. Kubernetes remains later work and is not needed to demonstrate the current API.

Native services depend on operator-maintained Ubuntu packages. Systemd services do not hold WSL alive; the Windows entry point launches one hidden lifetime helper that exits when the stack stops. Services are started on demand and bind loopback. Moving the runtime later requires explicit PostgreSQL migration and identity/key retention; it must preserve OIDC issuer/subject ownership or provide a deliberate identity migration rather than silently creating new owners.

## Verification and limits

See [native operator guide](../local-stack.md) and [F07 evidence](../f07-validation.md). This is local development with Keycloak `start-dev`, no remote TLS or availability claim. Clean restart is not backup/crash-recovery evidence. Temporal deployment and research execution follow their implementation ticket.
