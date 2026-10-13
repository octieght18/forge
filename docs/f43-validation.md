# F43 verification — 13 October 2026

F43 adds Argo CD applications and local and staging overlays. [D58](decision-log.md#d58--f43-argo-cd-gitops) records the boundary. The credential and image limits are in [gitops.md](gitops.md).

Argo CD was not installed. No cluster was reconciled. Bootstrap credentials are not in Git. The API image has no digest in `images.json`, and no signature was invented.

`TestGitOpsPinsExistingDigests` checks that both applications set `selfHeal: true`, that their source paths are the overlays rather than Terraform, and that every `sha256` under `deploy/gitops` is one of the digests already in `images.json`. The privileged-manifest check still passes.

Checked with Go 1.27.1:

```text
go test -count=1 -timeout 30s ./internal/policy -run 'TestGitOpsPinsExistingDigests|TestRepositoryManifestsAndNetworkAssumptions'
python3 scripts/check-docs.py
```
