# Cloud self-service deployment guide — F50

[D65](decision-log.md#d65--f50-cloud-deployment-guide) records the boundary. This guide sequences existing artifacts. It does **not** apply cloud resources.

Estimated spend for the unapplied EKS module shape is the F49 illustrative idle floor ≈ **$122.63 / month** in us-east-1 (price assumptions dated 2026-10-10). See [capacity-cost-model.md](capacity-cost-model.md).

## Prerequisites (cloud)

| Prerequisite | Status in this repository |
|---|---|
| AWS account, credentials, and spend approval | Operator-owned; not configured in this session |
| Terraform ≥ 1.16.5 and pinned `hashicorp/aws` 6.68.0 | Module present; `terraform` binary absent on this Linux validation VM |
| S3 remote state bucket + `backend.hcl` (gitignored) | Documented in [terraform-eks.md](terraform-eks.md); local state used only for prior validation |
| Domain name, TLS certificate, DNS delegation | Not purchased; [gateway.md](gateway.md) |
| Container registry digests/signatures for the API image | Digests only where already in `images.json`; F39 did not sign an image |
| Keycloak realm, model key, and operator secrets | Stay out of Git ([retention.md](retention.md)) |

Local substitute path (demonstrated elsewhere): native WSL stack or kind ([local-stack.md](local-stack.md), [kubernetes-local.md](kubernetes-local.md)), loopback `http://127.0.0.1:8081`.

## 1. Infrastructure bootstrap (do not run apply here)

1. Review `deploy/terraform/eks` (VPC, public/private subnets, IGW, one NAT, EKS control plane, one `t3.small` node scalable to two, private API endpoint).
2. Replace the local backend with operator S3 backend config **outside Git**.
3. `terraform init -backend-config=backend.hcl` then `terraform plan`. Review the plan against current AWS prices.
4. Only after explicit spend approval: `terraform apply`. **This guide's validation did not run apply.** Cleanup later: `terraform destroy` using that state (does not delete kind or the retained local database).

Identity notes from the module: cluster role trusts `eks.amazonaws.com` with `AmazonEKSClusterPolicy` only; node role trusts `ec2.amazonaws.com` with worker/CNI/ECR-read policies only. No administrator policy is attached in-module.

## 2. Application deployment

1. Install Argo CD into the target cluster (operator step). Keep the initial admin secret out of Git ([gitops.md](gitops.md)).
2. Apply Applications under `deploy/gitops/applications` pointing at `deploy/gitops/overlays/local` or `staging`. Both set `selfHeal: true` and `prune: true`. They do **not** point at the Terraform directory.
3. Confirm image pins: PostgreSQL, Keycloak, and proxy digests must already exist in `deploy/kubernetes/images.json`. Do not invent an API digest or cosign signature.
4. Optionally stage the Envoy gateway (`deploy/gateway/envoy.yaml`): ClusterIP Service, routes `/api/`, `/healthz`, `/readyz`, local 200 token/s limit. **Not applied** in prior tickets; TLS Secret `forge-gateway-tls` would be operator-created and not committed.
5. Run migrations with `forge-migrate` against the product database. Migrations are forward-only.
6. Start API, dispatcher, worker, and environment controller with automountServiceAccountToken false on the API pod pattern from the kind manifests. Developers still receive no kubeconfig.

## 3. Onboarding developers

1. Issue Keycloak user tokens (`developer` / `operator`). Service tokens remain 403 on product routes.
2. Follow [quickstart.md](quickstart.md) and [api-examples.md](api-examples.md) for the seven registration operations (no DELETE; 405).
3. Render templates, register, deploy, and request provision (`forge` CLI). Operator applies `ForgeEnvironment` via `scripts/environment.ps1`.
4. Submit research runs; inspect history/result. Model is `google/gemma-4-26b-a4b-it:free` with no paid fallback.

## 4. Promote, roll back, back up

| Action | Command / doc | Cloud note |
|---|---|---|
| Promote local → staging pin | `go run ./cmd/forge-promote --overlays deploy/gitops/overlays --from local --to staging --confirm promote-local-to-staging` | `--to cloud|eks|gke` **refused** until an environment exists ([promotion.md](promotion.md)) |
| Roll back regressed canary | [rollback.md](rollback.md); F45/F47 timings | Does not reverse migrations or unsend model calls |
| Backup / restore drill | [backup.md](backup.md); F46 dump/restore ms | Dump excludes Temporal history and in-process evidence |
| Incidents | [runbooks](runbooks/README.md), [incidents](incidents/README.md) | Simulated reports only |

## 5. Clean teardown

1. Remove Argo Applications / app namespaces (operator).
2. `terraform destroy` for the EKS stack state.
3. Rotate and delete operator-held secrets, DNS records, and certificates if any were created.
4. Confirm local kind and retained native data are untouched by cloud destroy.

## Production gaps (honest)

- G3–G5 are owner-reviewed with documented limits ([g3-validation.md](g3-validation.md), [g4-validation.md](g4-validation.md), [g5-validation.md](g5-validation.md)); G6 remains open.
- No applied EKS, no live Argo reconciliation, no DNS/TLS, no signed API image, no enforcing NetworkPolicy on kind.
- Rate limits and audit meters are per-process.
- Tenant model is one engineering team, not organization isolation.
- Cloud promotion remains refused when the environment was not created.

## Local validation of this guide's references

See [f50-validation.md](f50-validation.md). Checks cover GitOps pin tests, promotion refusal of cloud targets, gateway policy assumptions, docs links, and the F49 cost floor. Terraform apply, live Argo, and DNS/TLS were not run.
