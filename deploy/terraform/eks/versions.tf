terraform {
  required_version = "1.16.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.68.0"
    }
  }

  # Validation uses this local backend. Before any apply, switch to the S3
  # partial backend in docs/terraform-eks.md. Do not commit the state file.
  backend "local" {
    path = "terraform.tfstate"
  }
}

provider "aws" {
  region = var.region
}
