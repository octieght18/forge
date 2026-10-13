output "cluster_name" {
  value = aws_eks_cluster.forge.name
}

output "cluster_endpoint" {
  value = aws_eks_cluster.forge.endpoint
}

output "cluster_role_arn" {
  value = aws_iam_role.cluster.arn
}

output "node_role_arn" {
  value = aws_iam_role.node.arn
}
