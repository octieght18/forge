variable "region" {
  type    = string
  default = "us-east-1"
}

variable "cluster_name" {
  type    = string
  default = "forge-eks"
}

variable "kubernetes_version" {
  type        = string
  description = "EKS Kubernetes version. This session did not query the live catalog."
  default     = "1.33"
}

variable "availability_zones" {
  type    = list(string)
  default = ["us-east-1a", "us-east-1b"]
}

variable "vpc_cidr" {
  type    = string
  default = "10.40.0.0/16"
}
