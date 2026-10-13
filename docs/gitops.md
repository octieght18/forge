# Argo CD GitOps — F43

[D58](decision-log.md#d58--f43-argo-cd-gitops) records the boundary. The applications are in [deploy/gitops/applications](../deploy/gitops/applications). Argo CD was not installed. Nothing was reconciled onto a cluster, paid or local.

## Overlays

`deploy/gitops/overlays/local` targets namespace `forge-local`. `deploy/gitops/overlays/staging` targets namespace `forge-staging`. Both include the network documents, the Envoy manifest, and the image pin ConfigMap. There is no EKS overlay. That cluster was not created.

Each Application sets `selfHeal: true` and `prune: true`. That is the drift rule in Git. It was not observed, because the controller is not installed.

## Images

Digests already recorded in [images.json](../deploy/kubernetes/images.json) are copied into `forge-image-pins` for PostgreSQL, Keycloak, and the NGINX proxy. The local API image has no registry digest there, so the pin says so. No signature was added. F39 did not produce one, and this ticket does not invent one.

## State and bootstrap credentials

Infrastructure state is the Terraform state for `deploy/terraform/eks`. These Applications do not point at that directory. Application state is the Git overlay.

Argo CD's initial admin secret is created by its install and is read by the operator. The replacement password stays with the operator. This repository does not contain that secret, a token, or a kubeconfig. Installing Argo CD into the unpaid kind cluster was not done in this session either.
