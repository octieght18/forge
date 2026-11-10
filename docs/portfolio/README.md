# Forge portfolio and release package — F61

[D76](../decision-log.md#d76--f61-portfolio-and-release-package) records the boundary.

## Bundle index

| Artifact | Path |
|---|---|
| Architecture diagrams | [../diagrams/architecture.md](../diagrams/architecture.md) |
| ADR index | [../adr/README.md](../adr/README.md) |
| Architecture article | [../articles/secure-self-service-control-plane.md](../articles/secure-self-service-control-plane.md) |
| API / OpenAPI | [../api-examples.md](../api-examples.md), [../../internal/contract/openapi.json](../../internal/contract/openapi.json) |
| Threat model / failure analysis | [../threat-model.md](../threat-model.md) |
| Benchmarks | [../performance-developer-benchmark.md](../performance-developer-benchmark.md), [../f08-baseline.md](../f08-baseline.md) |
| Cost + ops evidence | [../operational-evidence-bundle.md](../operational-evidence-bundle.md), [../capacity-cost-model.md](../capacity-cost-model.md) |
| Simulated incidents | [../incidents/README.md](../incidents/README.md) |
| Runbooks | [../runbooks/README.md](../runbooks/README.md) |
| Demo script (no video file) | [../demo/five-minute-demo.md](../demo/five-minute-demo.md) |
| Resume — platform | [resume-platform-engineering.md](resume-platform-engineering.md) |
| Resume — AI infra | [resume-ai-infrastructure.md](resume-ai-infrastructure.md) |
| Quickstart | [../quickstart.md](../quickstart.md) |
| Cloud deployment guide | [../cloud-deployment-guide.md](../cloud-deployment-guide.md) |
| Release checklist | [../release-checklist.md](../release-checklist.md) |
| Module SBOM | [../validation/sbom-cyclonedx.json](../validation/sbom-cyclonedx.json) (generated; 117 components) |
| Supply-chain status | [../validation/f61-supply-chain-status.json](../validation/f61-supply-chain-status.json) |
| Platform review matrix | [../platform-review.md](../platform-review.md) |

## Supply chain

Follow [supply-chain.md](../supply-chain.md) / F39: `python3 scripts/supply-chain.py --check` writes the Go module CycloneDX SBOM from `go.sum`. If `syft`/`grype`/`trivy`/`cosign` or `FORGE_IMAGE` are absent, record **not-run** and **clean-scan: not-claimed**. **No image was signed in this session.**
