# SBOM, scan, and signature — F39

[D54](decision-log.md#d54--f39-sbom-scan-and-signature) records the boundary. Release review can see the Go module dependency list. An image scan and a signature run only when the tool and the image both exist.

## Module SBOM

`scripts/supply-chain.py --check` reads `go.sum` and writes a CycloneDX 1.5 document of module name and version. It does not copy checksum lines into the document. This is the module dependency list, not an image SBOM and not a vulnerability result.

## Image scan and signature

| Tool | When it runs |
|---|---|
| `syft` | `FORGE_IMAGE` is set and `syft` is on `PATH`. It writes a CycloneDX document for that image |
| `grype` or `trivy` | The same image is set and the scanner is on `PATH` |
| `cosign` | The image is set, `FORGE_COSIGN_KEY` is set, and `cosign` is on `PATH`. The script signs that image and then runs `cosign verify --key <key> <image>` |

The key stays with the operator. It is not committed. If any of those inputs is missing, the script prints `not-run` and `clean-scan: not-claimed`. A green CI job in that case is not a clean scan and is not a signed image.

## Triage

ahmad reviews scanner output before a release tag. A critical or high finding in the module or the image blocks that review until it is fixed or explicitly accepted in the decision log. Medium and low findings are recorded with the same review. No finding in this session was classified, because no scanner ran.

A signature proves the bytes match the operator's key. It does not prove the code is safe, the scan was clean, or the image is the one a cluster should run.
