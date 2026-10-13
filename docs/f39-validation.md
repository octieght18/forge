# F39 verification — 13 October 2026

F39 adds a module SBOM and a guarded image scan and signature. [D54](decision-log.md#d54--f39-sbom-scan-and-signature) records the boundary. The triage rule is in [supply-chain.md](supply-chain.md).

`syft`, `grype`, `trivy`, `cosign`, and Docker were not installed. No image was built. No image was signed. No scanner ran, so this record does not claim a clean scan. `plan` with those tools absent returns `image-scan: not-run`, `signed-image: not-run`, and `clean-scan: not-claimed`.

The module SBOM was generated from `go.sum`. The unit test checks a two-version fixture and checks that the repository `go.sum` produces more than ten components. The `--check` command wrote a temporary CycloneDX document with 117 components. It does not store checksums.

Checked with Python 3:

```text
python3 -m unittest discover -s scripts -p 'test_supply_chain.py'
python3 scripts/supply-chain.py --check
python3 scripts/check-docs.py
```

The `--check` run printed `module-sbom components=117`, `absent` for `syft`, `grype`, `trivy`, and `cosign`, then `signed-image: not-run` and `clean-scan: not-claimed`.
