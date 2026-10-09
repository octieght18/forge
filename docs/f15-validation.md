# F15 verification — 9 October 2026

F15 adds local service and MCP agent templates through `forge template`. [D29](decision-log.md#d29--f15-research-service-and-agent-templates) records the boundary. The original ticket's general service manifests and a second workload family are not available: the versioned API accepts only `research.v1` with `corpus.search` and `corpus.read`.

## Checks

`internal/template` renders both kinds, validates the workload and version documents against the embedded contract, and validates the agent run document. It checks the health, telemetry, and `ForgeEnvironment` shape, rejects an unknown kind and an invalid name, and refuses to replace existing files. The CLI test lists and renders without contacting an API. Reproduce with `go test -count=1 ./internal/template ./internal/cli`. Those package tests passed. The documentation check passed 43 Markdown documents and 318 relative links, and PowerShell parsing passed 7 scripts and 27 documented blocks.

The templates contain no credential material and no Helm, Kustomize, Compose, Nomad, or Argo CD instructions. `environment.json` keeps the placeholder workload and owner fields so it cannot be applied as a real environment by copying it unchanged.

## Material limits

Both templates share one research spec. The agent template's extra file is the sample run body, whose endpoint remains unimplemented. Health covers the platform process only. Telemetry names the correlation fields and does not deploy a collector. Operator environment apply stays [environment commands](execution-environments.md). Rendering a template does not register the workload.
