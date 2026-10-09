# Developer CLI — F14

The `forge` command is a JSON client for workload and version registration. It prints one JSON document per command. `request_id` is the API operation ID from `X-Request-ID`. Success goes to stdout and exits 0. API and transport failures go to stderr and exit 1. Local validation failures go to stderr and exit 2.

| Command | API call | Result |
|---|---|---|
| `register` | `POST /api/v1/workloads` | Creates one workload. Names are lowercase slugs. Registration is not idempotent. |
| `deploy` | `POST /api/v1/workloads/{workload_id}/versions` | Records an immutable research version. The result sets `execution` to `not_started` and `environment` to `not_requested`. |
| `status` | workload and version reads | Lists visible workloads, or reads one workload and its versions, or reads one version. |
| `delete` | `DELETE /api/v1/workloads/{workload_id}` | The current API rejects deletion with 405. The command reports that error, its operation ID, and leaves the workload in place. |

`deploy` does not create a namespace, start a worker, or call a model. Operator environment apply, status, and delete remain [environment commands](execution-environments.md). Build the CLI from a checkout with Go 1.27.1:

```powershell
Set-Location 'C:\Users\Owner\Downloads\Code\forge'
go build -o "$env:TEMP\forge-cli.exe" .\cmd\forge
```

Log in first, as in the [quickstart](quickstart.md). Then, from the repository root, with the private token file:

```powershell
$tokenFile = '\\wsl.localhost\Ubuntu\home\owner\.local\share\forge-native\tokens\access.json'
$forge = "$env:TEMP\forge-cli.exe"
$name = 'cli-' + [guid]::NewGuid().ToString('N')
$registered = & $forge register --token-file $tokenFile --name $name --description 'Developer CLI example' | ConvertFrom-Json
$deployed = & $forge deploy --token-file $tokenFile --workload $registered.workload.workload_id --spec .\internal\contract\examples\create-version.json | ConvertFrom-Json
& $forge status --token-file $tokenFile --workload $registered.workload.workload_id --version $deployed.version.version_id
& $forge delete --token-file $tokenFile --workload $registered.workload.workload_id
```

`register` and `deploy` each add one retained record. `delete` is expected to exit 1 with `error.code` `method_not_allowed`; a rejected delete does not remove the workload. The checked walkthrough is [cli-examples.ps1](../scripts/cli-examples.ps1):

```powershell
.\scripts\cli-examples.ps1 -TokenFile $tokenFile
```

It builds the CLI, registers a uniquely named workload, deploys the published [version example](../internal/contract/examples/create-version.json), reads that version, confirms delete is rejected, and reads the workload again. It prints IDs only. Use `--api` only for another loopback base URL; the default is `http://127.0.0.1:8081`. Do not put the access token on the command line. `--limit` is 1–100 and `--cursor` must be a cursor the API returned. `forge template` writes a local service or agent starting point without calling the API; see [workload templates](workload-templates.md). See [API examples](api-examples.md) for the HTTP behavior these commands call, and [F14 verification](f14-validation.md) for what was tested.
