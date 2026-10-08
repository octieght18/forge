# First local quickstart

This guide takes a clean checkout through the **native** Ubuntu WSL installation, which also supplies the source data and login helper for the accepted Kubernetes migration. For this PC's current copied kind stack, use the [Kubernetes guide](kubernetes-local.md) and `scripts/kubernetes-stack.ps1`; stop Kubernetes before starting this native alternative because both use the same loopback ports. Use [API examples](api-examples.md) to call all seven implemented operations; run/history/report/evidence paths in OpenAPI describe later work. Temporal workers/model execution are not implemented yet.

## 1. Check prerequisites

Use Windows, Git, PowerShell 7 and Ubuntu WSL2 with an unprivileged Linux user and systemd. If WSL is absent, follow [Microsoft's WSL installation guide](https://learn.microsoft.com/en-us/windows/wsl/install), including initial user setup/restart. Verify systemd following [Microsoft's systemd guide](https://learn.microsoft.com/en-us/windows/wsl/systemd). Do not use Linux `root` as the service user.

Inside Ubuntu, install Python 3.12+, an OpenJDK 21 runtime, PostgreSQL 18 binaries and **Linux Go 1.27.1**. A Windows Go installation cannot build the Linux service executables. Use the [official Go installation guide](https://go.dev/doc/install) and [downloads](https://go.dev/dl/) for the version in `go.mod`. PostgreSQL packages come from Ubuntu or the [official Apt repository](https://www.postgresql.org/download/linux/ubuntu/); PostgreSQL **18.6** is the validated version. Follow the [native guide](local-stack.md#prerequisites) for package commands and the pinned, checksum-verified Keycloak **26.8.0** archive. Startup downloads that archive when uncached; network access is needed for it and Go modules. Java is installed separately; the [Keycloak native distribution guide](https://www.keycloak.org/getting-started/getting-started-zip) documents that requirement.

PowerShell preflight, adjusting the first three variables to your installation:

```powershell
$distro = 'Ubuntu'
$serviceUser = 'owner'                 # Your Ubuntu username, not a Forge login
$linuxGo = '/usr/local/go/bin/go'      # Absolute Linux executable path
wsl.exe -l -v
wsl.exe -d $distro -u $serviceUser --exec id -un
wsl.exe -d $distro --exec systemctl --version
wsl.exe -d $distro --exec python3 --version
wsl.exe -d $distro --exec java -version
wsl.exe -d $distro --exec /usr/lib/postgresql/18/bin/postgres --version
wsl.exe -d $distro --exec $linuxGo version
```

Each check must succeed. On this already-configured PC, skip installation and use the existing checkout and saved Go selection; the portable Linux toolchain is documented in the native guide. Do not substitute the example `/usr/local/go/bin/go` path if it does not exist.

## 2. Clone and start

For a new checkout, from PowerShell:

```powershell
New-Item -ItemType Directory -Path "$HOME\Downloads\Code" -Force | Out-Null
Set-Location "$HOME\Downloads\Code"
git clone https://github.com/octieght18/forge.git
Set-Location forge
.\scripts\local-stack.ps1 up -Distro $distro -ServiceUser $serviceUser -Go $linuxGo
.\scripts\local-stack.ps1 status -Distro $distro -ServiceUser $serviceUser
Invoke-RestMethod 'http://127.0.0.1:8081/readyz'
```

If `forge` already exists, use it instead of cloning over it. For this PC's current installation:

```powershell
Set-Location 'C:\Users\Owner\Downloads\Code\forge'
$distro = 'Ubuntu'
$serviceUser = 'owner'
.\scripts\local-stack.ps1 up
```

Startup builds the API/migrator/login helper, explicitly migrates the product database, applies runtime grants, and waits for PostgreSQL, Keycloak and API readiness. It generates private local passwords, separate product/identity databases and three demo identities. Wait for **“Windows loopback connectivity verified.”** First installation takes longer than retained-data startup. `/readyz` should return `{"status":"ok"}`. API port **8081**, Keycloak **8082**, management **9002** and dedicated PostgreSQL **55436** must be available; startup does not kill unrelated port owners.

Running only `go run ./cmd/api` defaults to process health, without registration. Use this native entry point for the complete local API. The API has no homepage, Swagger UI or chat interface; open documentation in the repository and call the API with a client.

## 3. Log in and obtain an access token

The default state directory is `/home/<Linux user>/.local/share/forge-native`. For this PC, open the following file locally to obtain the generated `ahmad` password:

```text
\\wsl.localhost\Ubuntu\home\owner\.local\share\forge-native\secrets.json
```

`ahmad` is a developer with display name Ahmad; `second-owner` is another developer; `operator` can inspect all owners' records. `forge-admin` is the separate Keycloak infrastructure administrator. Use the selected account's generated password. Adjust distro/home paths for another machine. These local demo credentials are not committed defaults or an invitation to share the secrets file.

Run the login command and keep that terminal open:

```powershell
.\scripts\local-stack.ps1 login -Distro $distro -ServiceUser $serviceUser
```

Open **http://127.0.0.1:8083/login** in Firefox/another browser, finish Keycloak login within five minutes, and wait for **“Login complete.”** Start from the displayed URL, not a saved Keycloak authorization/callback URL. The helper handles state/cookie and S256 PKCE, exchanges the code, verifies the API access token and saves it privately. It exits after completion or the deadline, so port 8083 is normally closed between logins. Reopening/refreshing an old callback can show “can't connect” even after a token was saved. For a failed/expired attempt, rerun the command and begin a fresh login; an old authorization code cannot be reused.

In a **second PowerShell terminal**, set a private token path and call the API:

```powershell
$tokenFile = '\\wsl.localhost\Ubuntu\home\owner\.local\share\forge-native\tokens\access.json'
$token = (Get-Content -LiteralPath $tokenFile -Raw | ConvertFrom-Json).access_token
Invoke-RestMethod 'http://127.0.0.1:8081/api/v1/workloads' `
    -Headers @{ Authorization = "Bearer $token" }
```

The response contains `items` and `next_cursor`; a fresh developer may have an empty list. Tokens expire after **five minutes**. Repeat login and reload the token for subsequent calls; there is no saved refresh token. In Postman, choose **Authorization → Bearer Token** and use only the `access_token` value from the file. Product requests need `Authorization: Bearer <access_token>`; Keycloak's browser cookie, the callback code and an ID token are not API credentials.

For native Ubuntu, run `sudo python3 deploy/native/stack.py up --user "$USER" --go "$(command -v go)"`, then `~/.local/share/forge-native/bin/forge-login --token-file ~/.local/share/forge-native/tokens/access.json`. Use the Linux file path for a client; the issuer, browser URL and API remain the same. The [native guide](local-stack.md) describes service limits, data retention and teardown.

## 4. Register, update and read

In the authenticated PowerShell terminal, from the repository root:

```powershell
.\scripts\api-examples.ps1 -TokenFile $tokenFile
```

This calls all seven current API operations with the same payload files the contract validates: create/read/list workload, PATCH metadata with a fresh ETag, and create/read/list immutable version. Every invocation creates **one new synthetic workload and version** and retains them; it prints response statuses and IDs, without the token. The example uses the already-approved synthetic corpus IDs, not an installed document corpus. Registering a spec does not run research or call OpenRouter. Record the printed IDs for later requests. [API examples](api-examples.md) explain each request, pagination, errors and retries.

## 5. Stop and restart

```powershell
.\scripts\local-stack.ps1 stop -Distro $distro -ServiceUser $serviceUser
.\scripts\local-stack.ps1 up -Distro $distro -ServiceUser $serviceUser
```

Ordinary stop retains product records, identities, secrets and cursor key. Windows reboot or WSL shutdown requires `up` again. The driver never migrates on API startup alone; the native `up` sequence invokes the explicit migration command. Purge is a separate confirmed operator action, outside this quickstart.

## Troubleshooting

| Symptom | Action |
|---|---|
| Firefox cannot reach 8083 | Start a new `login`, keep its terminal open and use the fresh `/login` URL. The helper closes after successful login or five minutes. Check `tokens/access.json` expiry and an authenticated GET before assuming the saved token failed. |
| API returns 401 | Complete fresh login and reload `access_token`. Confirm exact issuer `http://127.0.0.1:8082/realms/forge` and audience `forge-api`; use an API access token. |
| API returns 404 for a workload | Check the UUID and current owner. Another developer's object also returns 404; an operator can inspect it. Run endpoints are not implemented. |
| POST returns 422 | Use the published schema/example and approved corpus IDs. Names are lowercase slugs; unsupported/unknown fields are rejected. |
| PATCH returns 428 or 412 | GET the workload, copy its complete quoted ETag into `If-Match`, then apply the intended change against that revision. |
| Startup fails or Windows cannot reach 8081/8082 | Verify prerequisites, `status`, readiness and ports in the native guide. Inspect private systemd diagnostics. `up` must finish Windows connectivity checks. |
| Clone works but Go build fails | Verify the **Linux** Go executable/version and module-network access; pass its absolute path through `-Go`. |

Use [repository navigation](repository-map.md) to find implementation/tests, [OpenAPI](../internal/contract/openapi.json) for schemas and [F09 validation](f09-validation.md) for observed clean-checkout coverage and limits.
