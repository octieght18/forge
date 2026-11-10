# Release checklist

Use before any tagged release or before an owner makes the repository public. Checking a box here does **not** close G3–G6.

## Hygiene

- [ ] `git status` clean; no `.env`, tokens, `*.pem`, Terraform state, or `.cursor/` paths staged
- [ ] [LICENSE](../LICENSE), [CONTRIBUTING.md](../CONTRIBUTING.md), [SECURITY.md](../SECURITY.md) present
- [ ] `python3 scripts/check-docs.py` passes
- [ ] `go test ./...` passes on the release revision (note any DB-required skips)
- [ ] `python3 scripts/supply-chain.py --check` produces/updates the module SBOM
- [ ] Image scan/sign recorded only if tools and `FORGE_IMAGE` existed; otherwise `not-run` / `clean-scan: not-claimed` ([supply-chain.md](supply-chain.md))

## Product honesty

- [ ] README and validation docs still state unapplied cloud, unsigned images, and open gates where true
- [ ] No new production SLO invented without measurements
- [ ] Incident reports remain labelled simulated if not from production
- [ ] Portfolio resumes cite only in-repo measurements ([portfolio](portfolio/) when present)

## Optional owner actions (explicit)

- [ ] Create a git tag
- [ ] Change GitHub repository visibility to public
- [ ] Attach release notes linking quickstart, threat model, benchmarks, cost model, and demo artifacts
