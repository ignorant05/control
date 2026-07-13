output "eks_cluster_name" {
  value = module.compute.cluster_name
}

output "ecr_repository_url" {
  value = module.compute.ecr_repository_url
}

output "rds_endpoint" {
  value = module.data.endpoint
}

output "redis_endpoint" {
  value = module.caching.primary_endpoint
}

output "github_actions_role_arn" {
  value = module.edge.github_actions_role_arn
}
