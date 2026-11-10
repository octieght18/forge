# F50 verification — 10 November 2026

F50 publishes the cloud self-service deployment guide. [D65](decision-log.md#d65--f50-cloud-deployment-guide) records the boundary. Guide: [cloud-deployment-guide.md](cloud-deployment-guide.md).

## Checks run

```text
go test -count=1 -timeout 30s ./internal/policy -run 'TestGitOpsPinsExistingDigests|TestRepositoryManifestsAndNetworkAssumptions|TestGatewayIsLocalAndUnauthenticatedPathStaysOnTheAPI'
go test -count=1 ./internal/delivery ./cmd/forge-promote
python3 scripts/check-docs.py
```

Expected: GitOps applications self-heal and pin only existing digests; gateway remains unprivileged ClusterIP without keys; promote copies local→staging in tests and refuses cloud/eks/gke; docs links resolve.

## Not run

- `terraform apply` / `terraform destroy`
- Argo CD install or live reconcile
- DNS purchase, certificate issuance, TLS handshake
- Envoy process start
- Any AWS API call

`terraform` was not installed on this validation VM. Estimated spend cites F49 only. G5 stays open.
