# F07 — Native local stack

The owner instructed **“don't containerize anything”** on 6 October 2026 and accepted native PostgreSQL, Keycloak and Go API services inside Ubuntu WSL, managed by systemd with a PowerShell entry point. This supersedes the original F07 Kubernetes title and the earlier Compose-first deployment decision for this stack. No Docker, container image or Kubernetes cluster is installed by F07. Temporal and its worker follow the workflow ticket.

The owner also accepted generated private secrets, separate identity/product databases and roles, two developer demo identities plus an operator, authorization-code/S256 PKCE login, and retained data during ordinary stop/teardown. All services bind loopback; this is an operator's local development installation with no high availability or production identity claim.

## Prerequisites

Use Windows with Ubuntu WSL2 and systemd enabled, or native Ubuntu with systemd. Install Python 3.12+, PostgreSQL 18.6 and a supported OpenJDK 21 runtime. Go 1.27.1 must be available to build the repository. This PC already had PostgreSQL and portable Go; F07 installed OpenJDK 21. The [native Keycloak distribution](https://www.keycloak.org/getting-started/getting-started-zip) and [supported Java/PostgreSQL versions](https://www.keycloak.org/server/supported-configurations) define its prerequisites.

Ubuntu prerequisite example, run deliberately as an operator:

```sh
sudo apt-get update
sudo apt-get install --no-install-recommends python3 openjdk-21-jre-headless postgresql-18
```

Ubuntu releases without PostgreSQL 18 in their own repositories need the [official PostgreSQL Apt repository](https://www.postgresql.org/download/linux/ubuntu/). Installation scripts do not change the host's package repositories automatically. Verify the installed PostgreSQL version before use; an existing system PostgreSQL instance remains separate from Forge's dedicated cluster on port 55436.

Keycloak is pinned to **26.8.0**, and its official release archive is verified against SHA-256 `9e41da899f838a58cd510fc98ed4f7cadc715aed5683e42aca20a0c9a2a3980a` before extraction. A cached archive can be supplied; otherwise startup downloads it from the [official release](https://github.com/keycloak/keycloak/releases/tag/26.8.0). Download/extraction and the first realm/schema initialization take longer than later starts. Application dependencies remain pinned by `go.mod`/`go.sum`; system packages remain operator-maintained and must be patched deliberately.

## Start and stop from Windows

From the repository root:

```powershell
.\scripts\local-stack.ps1 up
.\scripts\local-stack.ps1 status
.\scripts\local-stack.ps1 stop
```

Use `-Distro` and `-ServiceUser` if the defaults `Ubuntu`/`owner` differ. For this PC's initial portable Go setup:

```powershell
.\scripts\local-stack.ps1 up -Go /mnt/c/Users/Owner/Documents/Codex/2026-10-04/in/tools/go-linux/go/bin/go
```

The selected Go executable is retained in private configuration for later startup commands when `go` is absent from WSL's PATH. `-KeycloakArchive` accepts an existing Windows archive path. Startup requires WSL root only for lifecycle management: PostgreSQL, Java and the API execute as the selected unprivileged service user. It does not enable automatic boot/login startup.

The driver builds binaries, starts a dedicated PostgreSQL cluster, bootstraps separate credentials/databases, explicitly runs `forge-migrate` and runtime grants, starts Keycloak, waits for its readiness and realm discovery, then starts the API and verifies readiness. Repeated startup retains existing credentials, identities, keys and data. A busy port owned by another process fails startup instead of terminating that process. API startup still performs no migration. A startup failure can leave infrastructure running for inspection; normal `stop` is the recovery/teardown command.

The PowerShell entry point verifies the API and exact issuer through **Windows** loopback after Linux readiness. It launches a hidden, unprivileged WSL lifetime helper; one helper per installation holds WSL while its native units run and exits after stop. [Systemd services alone do not keep WSL alive](https://learn.microsoft.com/en-us/windows/wsl/systemd). A Windows reboot or explicit WSL shutdown stops this local installation; use `up` again. Linux users can run `sudo python3 deploy/native/stack.py up --user "$USER" --go "$(command -v go)"` directly and do not need the WSL helper.

## Addresses, probes and limits

| Component | Loopback address | Readiness | systemd limits |
| --- | --- | --- | --- |
| Forge API | `http://127.0.0.1:8081` | `/readyz` checks accessible product tables; `/healthz` is process liveness | 256 MiB memory, one CPU equivalent |
| Keycloak | `http://127.0.0.1:8082` | Management `http://127.0.0.1:9002/health/ready`, plus realm discovery | 1,536 MiB memory, one CPU equivalent; JVM heap 256–768 MiB |
| Dedicated PostgreSQL | `127.0.0.1:55436` | `pg_isready`, then authenticated migration/schema checks | 1 GiB memory, one CPU equivalent |
| One-shot login helper | `http://127.0.0.1:8083/login` | Runs only during requested login | Five-minute deadline, bounded HTTP/exchange timeouts |

The service caps are local safeguards, not F08 capacity/SLO measurements. Units use a private umask, no privilege escalation, no automatic restart and a 30-second stop limit. API graceful drain still follows F03's ten-second grace. PostgreSQL uses fast orderly shutdown. Inspect native unit names using `status`, then `journalctl -u <unit>` locally. Logs must stay private; never publish connection strings or tokens.

## Identity and login

The imported `forge` realm has developer accounts `ahmad` (display name Ahmad) and `second-owner`, and an `operator` account. The bootstrap administrator is `forge-admin`; it is infrastructure administration, not a Forge product operator. Passwords are random local secrets, not committed defaults. Registration/password grants/implicit grants are disabled; the public `forge-local-login` client requires S256 PKCE and the exact redirect `http://127.0.0.1:8083/callback`. Audience and client roles match F06, with RS256 and five-minute access tokens.

Startup [realm import preserves an existing realm](https://www.keycloak.org/server/importExport). Editing the committed template does not reset existing users, passwords or roles. Subsequent identity changes are explicit Keycloak administrator operations, not re-import/overwrite actions. Newly generated fixture passwords are initial values; if changed in Keycloak, the original fixture secret entry is no longer an active password.

Read the desired demo password privately from the retained `secrets.json` file; do not paste it into chat or logs. Then run:

```powershell
.\scripts\local-stack.ps1 login
```

Open the displayed loopback URL and log in. The helper binds state to a private browser cookie, exchanges the code using S256 PKCE, verifies the real signed API access token and role, and saves only the access token and attribution/expiry in `tokens/access.json`. It never prints tokens or saves refresh tokens. Wrong state/issuer, absent cookie and replayed callbacks fail. Access expires after five minutes; repeat login when needed. Treat the private token file as a credential.

## Retained data and teardown

Default location inside Ubuntu: **`/home/owner/.local/share/forge-native`** (or the selected user's corresponding home). On Windows, view it through `\\wsl.localhost\Ubuntu\home\owner\.local\share\forge-native`. The directory is private (0700); generated secrets/tokens are 0600.

| Retained item | Purpose |
| --- | --- |
| `postgres/` | Dedicated cluster with `forge` product data and `keycloak` persistent identity data |
| `secrets.json`, `postgres-password` | Distinct administrator, migrator, runtime, identity and synthetic-user secrets |
| `cursor.key`, `corpus-policy.json` | Stable cursor signing key and operator-managed corpus ID approval |
| `keycloak-26.8.0/` | Verified native distribution and private initial realm import |
| `bin/`, `runtime.py`, `go-toolchain.json` | Built native binaries, launcher and selected Go executable |
| `tokens/` | Expiring private access-token files |
| `installation.json` | Installation identity guard for lifecycle operations |

Normal `stop` removes running services, releases their ports and retains every item above. It does not stop unrelated Ubuntu services or shut down the entire WSL distro. Restart proof compares persistent identities, workload/version IDs and cursor key. This is clean local persistence evidence, not backup/crash recovery or an availability SLO.

Deletion is a distinct operator action, unavailable through normal PowerShell `up`/`stop`. Back up first, stop the installation, then explicitly invoke the native driver's `purge` action with `--confirm` equal to the exact absolute state directory. It refuses an active stack, a mismatched installation marker, a home/system directory or symlinked target/ancestor. F07 validation does not delete this PC's retained product data.

## Validation

The [native smoke program](../deploy/native/smoke.py) exercises the actual authorization page/form, helper callback/code exchange and signed-token verification, not a mock issuer or password-grant shortcut. It checks wrong/missing PKCE denial, disabled password grant, workload/version creation/read, second-owner denials, operator inspection and unauthenticated rejection. `seed`, followed by stop/up and `verify`, checks retained workload/version records, the same three identity subjects and the stable cursor key. It uses synthetic local accounts and never makes a model call.

See [F07 evidence](f07-validation.md) for actual results and limitations. The native development realm uses local HTTP and `start-dev`; remote exposure, production TLS/identity operations, backups, retention, worker/Temporal durability and Kubernetes remain outside this ticket.
