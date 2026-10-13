# F41 verification — 13 October 2026

F41 adds one EKS module and does not apply it. [D56](decision-log.md#d56--f41-eks-terraform) records the boundary. G4 is not marked done. F02 remains local kind. The cost and state notes are in [terraform-eks.md](terraform-eks.md).

Terraform 1.16.5 was downloaded and its zip matched the published SHA-256. `terraform init -backend=false` and `terraform validate` succeeded for `deploy/terraform/eks`. `terraform plan` was not run. `terraform apply` was not run. No AWS credentials were configured. No resources were created and no spend occurred.

The module contains no `google_` or `azurerm_` resources and no `AdministratorAccess` policy. The active backend is local. The state path is gitignored.

Checked:

```text
python3 -m unittest discover -s deploy/terraform -p 'test_*.py'
terraform -chdir=deploy/terraform/eks init -backend=false
terraform -chdir=deploy/terraform/eks validate
```
