# F61 verification — 10 November 2026

F61 packages the portfolio release index and supply-chain SBOM. [D76](decision-log.md#d76--f61-portfolio-and-release-package) records the boundary. Index: [portfolio/README.md](portfolio/README.md).

## Checks

```text
python3 scripts/supply-chain.py --check
python3 scripts/check-docs.py
```

Image scan/sign: not-run when tools/image absent (same honesty as F39). No image signed.
