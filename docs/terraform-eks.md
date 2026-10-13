# EKS plan — F41

[D56](decision-log.md#d56--f41-eks-terraform) records the boundary. F02 selected local kind, not a paid cluster. G4 is not marked done. This page is one cloud target, Amazon EKS. Google GKE is not in this change.

## What was not done

`terraform apply` was not run. No AWS account was configured. No VPC, cluster, node, NAT gateway, or address was created. No spend occurred. The AWS price list was not queried. `kubernetes_version` defaults to `1.33` and was not checked against the live EKS catalog.

An apply would be billable. The resources in the module are a VPC, two public subnets, two private subnets, an internet gateway, one NAT gateway and its public IPv4 address, an EKS control plane, and one `t3.small` node that can scale to two. Review that list and the current AWS price list before spending. Cleanup, after a future apply, is `terraform destroy` in `deploy/terraform/eks` using the state for this stack. That command does not delete the local kind cluster or the retained product database.

## Identity and permissions

The cluster role can be assumed by `eks.amazonaws.com` and attaches only `AmazonEKSClusterPolicy`. The node role can be assumed by `ec2.amazonaws.com` and attaches only `AmazonEKSWorkerNodePolicy`, `AmazonEKS_CNI_Policy`, and `AmazonEC2ContainerRegistryReadOnly`. There is no administrator policy. The API endpoint is private. No public endpoint was opened. Nothing here was created, so that endpoint does not exist.

## Remote state

Validation uses the local backend `deploy/terraform/eks/terraform.tfstate`. That file is gitignored. Do not commit it.

Before any apply, an operator replaces the local backend with an S3 backend configured outside Git:

```hcl
terraform {
  backend "s3" {}
}
```

```text
bucket       = operator-chosen-name
key          = forge/eks/terraform.tfstate
region       = us-east-1
use_lockfile = true
```

Pass that file with `terraform init -backend-config=backend.hcl`. `backend.hcl` is gitignored. It must not contain access keys. Credentials stay in the operator's AWS profile or environment, not in the repository.

Providers are pinned to Terraform 1.16.5 and `hashicorp/aws` 6.68.0.
