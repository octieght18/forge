# Workload templates — F15

`forge template` writes a local starting point for a research service, a read-only MCP agent, or a resource-constrained research job. It does not call the API, create a workload, or apply Kubernetes. All three kinds use the only supported contract, `research.v1`, with the tools `corpus.search` and `corpus.read`. The agent adds a sample run request. The job records the controller profile `small-v1`. None of the files contains credentials, and none introduces Helm, Kustomize, Compose, or another orchestrator.

Generate them from a checkout:

```powershell
Set-Location 'C:\Users\Owner\Downloads\Code\forge'
go build -o "$env:TEMP\forge-cli.exe" .\cmd\forge
$forge = "$env:TEMP\forge-cli.exe"
& $forge template list
& $forge template render --kind service --name research-service --out "$env:TEMP\forge-service-template"
& $forge template render --kind agent --name research-agent --out "$env:TEMP\forge-agent-template"
& $forge template render --kind job --name research-job --out "$env:TEMP\forge-job-template"
```

`--name` must be a lowercase slug of 1–63 characters. The command refuses to replace an existing file in `--out`.

| File | Contents |
|---|---|
| `workload.json` | Registration body. The descriptions differ by kind; the name is the one you passed. |
| `version.json` | The published immutable research spec, including the approved synthetic corpus IDs. |
| `run.json` | Agent only. A valid future run body whose workload and version IDs are synthetic examples. |
| `resources.json` | Job only. The `small-v1` quota and container maximum already enforced by the controller. Rendering does not apply them. |
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

`environment.json` is not applied by this command. After registration, an operator copies the real workload ID and owner into the existing [environment command](execution-environments.md). Developers do not receive a kubeconfig. Replace the synthetic IDs in `run.json` only when run submission exists; posting it today is outside the available API. `resources.json` describes `small-v1` and does not create a quota. See [F15 verification](f15-validation.md) and [F19 measurement](f19-validation.md).
