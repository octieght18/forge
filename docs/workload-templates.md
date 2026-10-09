# Workload templates — F15

`forge template` writes a local starting point for a research service or a read-only MCP agent. It does not call the API, create a workload, or apply Kubernetes. Both kinds use the only supported contract, `research.v1`, with the tools `corpus.search` and `corpus.read`. The agent adds a sample run request. Neither file contains credentials, and neither introduces Helm, Kustomize, Compose, or another orchestrator.

Generate them from a checkout:

```powershell
Set-Location 'C:\Users\Owner\Downloads\Code\forge'
go build -o "$env:TEMP\forge-cli.exe" .\cmd\forge
$forge = "$env:TEMP\forge-cli.exe"
& $forge template list
& $forge template render --kind service --name research-service --out "$env:TEMP\forge-service-template"
& $forge template render --kind agent --name research-agent --out "$env:TEMP\forge-agent-template"
```

`--name` must be a lowercase slug of 1–63 characters. The command refuses to replace an existing file in `--out`.

| File | Contents |
|---|---|
| `workload.json` | Registration body. The service and agent descriptions differ; the name is the one you passed. |
| `version.json` | The published immutable research spec, including the approved synthetic corpus IDs. |
| `run.json` | Agent only. A valid future run body whose workload and version IDs are synthetic examples. |
| `health.json` | Platform `GET /healthz` and `GET /readyz`. A workload-specific probe is `not_exposed`. |
| `telemetry.json` | Correlate `request_id`, `workload_id`, and `version_id`. Omit credentials, raw prompts, and corpus text. No collector is included. |
| `environment.json` | Shape of the existing `ForgeEnvironment` manifest with `<workloadID>`, `<issuer>`, and `<subject>` placeholders and profile `small-v1`. |
| `availability.json` | `register` and `deploy` are available. `run` is not available. Environment apply is operator-only. |

Register and deploy the generated files with the [developer CLI](developer-cli.md) after login:

```powershell
$registered = & $forge register --token-file $tokenFile --name research-agent --description 'Read-only MCP research agent using corpus.search and corpus.read' | ConvertFrom-Json
$deployed = & $forge deploy --token-file $tokenFile --workload $registered.workload.workload_id --spec "$env:TEMP\forge-agent-template\version.json" | ConvertFrom-Json
```

Use the description from the generated `workload.json` if you choose a different name. `deploy` records the version and does not start execution.

`environment.json` is not applied by this command. After registration, an operator copies the real workload ID and owner into the existing [environment command](execution-environments.md). Developers do not receive a kubeconfig. Replace the synthetic IDs in `run.json` only when run submission exists; posting it today is outside the available API. See [F15 verification](f15-validation.md).
